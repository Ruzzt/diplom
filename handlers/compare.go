package handlers

import (
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ComparePage отдаёт страницу /compare для сравнения двух моделей распознавания
// лиц: готовой @vladmandic/face-api (ResNet-34) и собственной CNN, обученной
// в ml/ на LFW.
func ComparePage(c *gin.Context) {
	c.HTML(http.StatusOK, "compare.html", gin.H{
		"title":           "Сравнение моделей распознавания",
		"customModelReady": FaceModelReady(),
	})
}

// CompareModelsRequest — фронт отправляет сюда сразу обе модели:
//  - cropped_image_b64 — лицо, обрезанное face-api до квадрата (нужно нашей модели);
//  - face_api_descriptor — 128-D дескриптор face-api от того же лица;
// и эти данные для двух изображений.
type CompareModelsRequest struct {
	Image1 ComparePersonInput `json:"image1"`
	Image2 ComparePersonInput `json:"image2"`
}

type ComparePersonInput struct {
	CroppedImageB64    string    `json:"cropped_image_b64"`
	FaceApiDescriptor  []float64 `json:"face_api_descriptor"`
}

type ModelResult struct {
	Distance       float64   `json:"distance"`
	Threshold      float64   `json:"threshold"`
	Match          bool      `json:"match"`
	InferenceMs1   float64   `json:"inference_ms_1"`
	InferenceMs2   float64   `json:"inference_ms_2"`
	Embedding1     []float32 `json:"embedding_1,omitempty"`
	Embedding2     []float32 `json:"embedding_2,omitempty"`
	Error          string    `json:"error,omitempty"`
}

type CompareModelsResponse struct {
	FaceApi     ModelResult `json:"face_api"`
	CustomModel ModelResult `json:"custom_model"`
}

// CompareModels — POST /api/compare-models. Принимает два лица + их дескрипторы
// от face-api, прогоняет картинки через собственную модель и возвращает
// сравнительные метрики для обеих моделей.
func CompareModels(c *gin.Context) {
	var req CompareModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Невалидный запрос: " + err.Error()})
		return
	}

	resp := CompareModelsResponse{}

	// --- face-api: считаем расстояние между готовыми дескрипторами ---
	const faceApiThreshold = 0.6 // стандартный порог @vladmandic/face-api
	if len(req.Image1.FaceApiDescriptor) == FaceModelEmbeddingDim &&
		len(req.Image2.FaceApiDescriptor) == FaceModelEmbeddingDim {
		d := euclideanDistance(req.Image1.FaceApiDescriptor, req.Image2.FaceApiDescriptor)
		resp.FaceApi = ModelResult{
			Distance:  d,
			Threshold: faceApiThreshold,
			Match:     d < faceApiThreshold,
		}
	} else {
		resp.FaceApi.Error = "face-api дескриптор не предоставлен или некорректен"
	}

	// --- Собственная модель: инференс на каждом фото + расстояние ---
	const customThreshold = 1.2456 // best_threshold из ml/checkpoints/eval.json (Путь B)
	if !FaceModelReady() {
		resp.CustomModel.Error = "Собственная модель не загружена"
	} else if req.Image1.CroppedImageB64 == "" || req.Image2.CroppedImageB64 == "" {
		resp.CustomModel.Error = "Не получены обрезанные изображения"
	} else {
		emb1, t1, err := FaceModelInference(req.Image1.CroppedImageB64)
		if err != nil {
			resp.CustomModel.Error = "Ошибка инференса первого фото: " + err.Error()
			c.JSON(http.StatusOK, resp)
			return
		}
		emb2, t2, err := FaceModelInference(req.Image2.CroppedImageB64)
		if err != nil {
			resp.CustomModel.Error = "Ошибка инференса второго фото: " + err.Error()
			c.JSON(http.StatusOK, resp)
			return
		}
		d := euclideanDistance32(emb1, emb2)
		resp.CustomModel = ModelResult{
			Distance:     d,
			Threshold:    customThreshold,
			Match:        d < customThreshold,
			InferenceMs1: t1,
			InferenceMs2: t2,
			Embedding1:   emb1,
			Embedding2:   emb2,
		}
	}

	c.JSON(http.StatusOK, resp)
}

func euclideanDistance32(a, b []float32) float64 {
	if len(a) != len(b) {
		return math.Inf(1)
	}
	var sum float64
	for i := range a {
		d := float64(a[i] - b[i])
		sum += d * d
	}
	return math.Sqrt(sum)
}

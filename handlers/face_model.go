package handlers

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	ort "github.com/yalue/onnxruntime_go"
)

// Собственная модель распознавания лиц, обученная в ml/ на LFW.
// Подгружается один раз при старте сервера через InitFaceModel.
// Используется страницей /compare для сопоставления с face-api.

const (
	FaceModelInputSize    = 112
	FaceModelEmbeddingDim = 128
)

var (
	faceModelOnce    sync.Once
	faceModelLoadErr error

	faceModelSession   *ort.DynamicAdvancedSession
	faceModelInputName  = "input"
	faceModelOutputName = "embedding"
)

// defaultOnnxRuntimeLib возвращает наиболее вероятный путь к libonnxruntime
// для текущей платформы. Можно переопределить переменной окружения ONNXRUNTIME_LIB.
func defaultOnnxRuntimeLib() string {
	if p := os.Getenv("ONNXRUNTIME_LIB"); p != "" {
		return p
	}
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "/opt/homebrew/lib/libonnxruntime.dylib"
		}
		return "/usr/local/lib/libonnxruntime.dylib"
	case "linux":
		return "/usr/lib/x86_64-linux-gnu/libonnxruntime.so"
	}
	return ""
}

// InitFaceModel инициализирует среду ONNX Runtime и грузит ONNX-модель.
// Вызывается из main.go при старте сервера.
func InitFaceModel(modelPath string) error {
	faceModelOnce.Do(func() {
		lib := defaultOnnxRuntimeLib()
		if lib == "" {
			faceModelLoadErr = errors.New("не найден путь к libonnxruntime, " +
				"установи переменную ONNXRUNTIME_LIB")
			return
		}
		if _, err := os.Stat(lib); err != nil {
			faceModelLoadErr = fmt.Errorf("libonnxruntime не найден по пути %s: %w "+
				"(установи: brew install onnxruntime)", lib, err)
			return
		}
		ort.SetSharedLibraryPath(lib)
		if err := ort.InitializeEnvironment(); err != nil {
			faceModelLoadErr = fmt.Errorf("ort.InitializeEnvironment: %w", err)
			return
		}
		if _, err := os.Stat(modelPath); err != nil {
			faceModelLoadErr = fmt.Errorf("модель %s не найдена: %w", modelPath, err)
			return
		}
		session, err := ort.NewDynamicAdvancedSession(
			modelPath,
			[]string{faceModelInputName},
			[]string{faceModelOutputName},
			nil,
		)
		if err != nil {
			faceModelLoadErr = fmt.Errorf("ort.NewDynamicAdvancedSession: %w", err)
			return
		}
		faceModelSession = session
		log.Printf("[face_model] модель загружена: %s (libonnxruntime: %s)", modelPath, lib)
	})
	return faceModelLoadErr
}

// FaceModelReady возвращает true, если модель готова принимать инференс.
func FaceModelReady() bool {
	return faceModelLoadErr == nil && faceModelSession != nil
}

// decodeBase64Image принимает строку вида "data:image/jpeg;base64,..." или
// чистую base64-строку и возвращает декодированный image.Image.
func decodeBase64Image(b64 string) (image.Image, error) {
	if idx := strings.Index(b64, ","); idx >= 0 {
		b64 = b64[idx+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64: %w", err)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	return img, nil
}

// imageToTensor приводит картинку к виду, который ждёт обученная модель:
//   - ресайз до 112×112
//   - RGB-порядок каналов
//   - нормализация (pixel/255 - 0.5) / 0.5  -> диапазон [-1, 1]
//   - формат NCHW: float32[1, 3, 112, 112]
//
// Точно соответствует transforms из ml/dataset.py:build_eval_transform.
func imageToTensor(img image.Image) []float32 {
	resized := imaging.Resize(img, FaceModelInputSize, FaceModelInputSize, imaging.Linear)
	planeSize := FaceModelInputSize * FaceModelInputSize
	out := make([]float32, 3*planeSize)
	bounds := resized.Bounds()
	for y := 0; y < FaceModelInputSize; y++ {
		for x := 0; x < FaceModelInputSize; x++ {
			r, g, b, _ := resized.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			// RGBA() возвращает 16-битные значения; делим на 65535 -> [0..1]
			rf := float32(r) / 65535.0
			gf := float32(g) / 65535.0
			bf := float32(b) / 65535.0
			idx := y*FaceModelInputSize + x
			out[0*planeSize+idx] = (rf - 0.5) / 0.5
			out[1*planeSize+idx] = (gf - 0.5) / 0.5
			out[2*planeSize+idx] = (bf - 0.5) / 0.5
		}
	}
	return out
}

// FaceModelInference прогоняет одну картинку через свою ONNX-модель и
// возвращает 128-мерный L2-нормализованный embedding плюс время инференса в мс.
func FaceModelInference(b64Image string) ([]float32, float64, error) {
	if !FaceModelReady() {
		return nil, 0, errors.New("модель не инициализирована")
	}

	img, err := decodeBase64Image(b64Image)
	if err != nil {
		return nil, 0, err
	}
	inputData := imageToTensor(img)

	inputShape := ort.NewShape(1, 3, FaceModelInputSize, FaceModelInputSize)
	inputTensor, err := ort.NewTensor(inputShape, inputData)
	if err != nil {
		return nil, 0, fmt.Errorf("new input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	outputShape := ort.NewShape(1, int64(FaceModelEmbeddingDim))
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, 0, fmt.Errorf("new output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	start := time.Now()
	if err := faceModelSession.Run(
		[]ort.Value{inputTensor},
		[]ort.Value{outputTensor},
	); err != nil {
		return nil, 0, fmt.Errorf("session.Run: %w", err)
	}
	elapsed := float64(time.Since(start).Microseconds()) / 1000.0

	embedding := make([]float32, FaceModelEmbeddingDim)
	copy(embedding, outputTensor.GetData())
	return embedding, elapsed, nil
}

// DestroyFaceModel освобождает ресурсы ONNX Runtime (вызывать на shutdown).
func DestroyFaceModel() {
	if faceModelSession != nil {
		faceModelSession.Destroy()
		faceModelSession = nil
	}
	_ = ort.DestroyEnvironment()
}

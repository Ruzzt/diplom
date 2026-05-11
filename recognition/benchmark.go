package recognition

import (
	"time"
)

// BenchmarkResult — результат сравнения двух методов распознавания
type BenchmarkResult struct {
	// Нейросетевой метод (face-api.js)
	NeuralDistance float64 `json:"neural_distance"`
	NeuralMatch   bool    `json:"neural_match"`
	NeuralTimeMs  float64 `json:"neural_time_ms"`

	// Геометрический метод (собственный)
	GeometricEuclidean    float64 `json:"geometric_euclidean"`
	GeometricCosine       float64 `json:"geometric_cosine"`
	GeometricMatchEuclid  bool    `json:"geometric_match_euclid"`
	GeometricMatchCosine  bool    `json:"geometric_match_cosine"`
	GeometricTimeMs       float64 `json:"geometric_time_ms"`

	// Пороги
	NeuralThreshold          float64 `json:"neural_threshold"`
	GeometricThresholdEuclid float64 `json:"geometric_threshold_euclid"`
	GeometricThresholdCosine float64 `json:"geometric_threshold_cosine"`
}

const (
	NeuralThreshold          = 0.6
	GeometricThresholdEuclid = 0.15  // Подобран экспериментально
	GeometricThresholdCosine = 0.005 // Подобран экспериментально
)

// RunBenchmark выполняет сравнение двух методов для одной пары дескрипторов
func RunBenchmark(
	neuralA, neuralB []float64,
	landmarksA, landmarksB []Landmark,
) BenchmarkResult {
	result := BenchmarkResult{
		NeuralThreshold:          NeuralThreshold,
		GeometricThresholdEuclid: GeometricThresholdEuclid,
		GeometricThresholdCosine: GeometricThresholdCosine,
	}

	// Нейросетевой метод
	startNeural := time.Now()
	if len(neuralA) > 0 && len(neuralB) > 0 {
		sum := 0.0
		for i := range neuralA {
			if i >= len(neuralB) {
				break
			}
			d := neuralA[i] - neuralB[i]
			sum += d * d
		}
		result.NeuralDistance = squareRoot(sum)
		result.NeuralMatch = result.NeuralDistance < NeuralThreshold
	}
	result.NeuralTimeMs = float64(time.Since(startNeural).Microseconds()) / 1000.0

	// Геометрический метод
	startGeo := time.Now()
	if len(landmarksA) >= 68 && len(landmarksB) >= 68 {
		descA := ComputeGeometricDescriptor(landmarksA)
		descB := ComputeGeometricDescriptor(landmarksB)

		if descA != nil && descB != nil {
			result.GeometricEuclidean = CompareGeometric(descA, descB)
			result.GeometricCosine = CompareGeometricCosine(descA, descB)
			result.GeometricMatchEuclid = result.GeometricEuclidean < GeometricThresholdEuclid
			result.GeometricMatchCosine = result.GeometricCosine < GeometricThresholdCosine
		}
	}
	result.GeometricTimeMs = float64(time.Since(startGeo).Microseconds()) / 1000.0

	return result
}

func squareRoot(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Метод Ньютона
	z := x / 2
	for i := 0; i < 20; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}

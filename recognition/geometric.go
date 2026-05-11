// Package recognition реализует собственный геометрический метод распознавания лиц.
// Метод основан на вычислении нормализованных расстояний между 68 ключевыми точками лица.
package recognition

import (
	"math"
)

// Landmark — точка лица (x, y в нормализованных координатах 0..1)
type Landmark struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// GeometricDescriptor — вектор геометрических признаков лица.
// Содержит 18 нормализованных расстояний между ключевыми точками.
type GeometricDescriptor []float64

// ComputeGeometricDescriptor вычисляет геометрический дескриптор лица
// на основе 68 ключевых точек (landmarks).
//
// Признаки (18 штук):
//  1. Расстояние между глазами (межзрачковое)
//  2. Ширина левого глаза
//  3. Ширина правого глаза
//  4. Высота левого глаза
//  5. Высота правого глаза
//  6. Длина носа (от переносицы до кончика)
//  7. Ширина носа
//  8. Ширина рта
//  9. Высота рта
//  10. Расстояние от носа до рта
//  11. Расстояние от рта до подбородка
//  12. Ширина лица (скулы)
//  13. Высота лица (лоб до подбородка)
//  14. Отношение ширины лица к высоте
//  15. Расстояние от левого глаза до кончика носа
//  16. Расстояние от правого глаза до кончика носа
//  17. Ширина левой брови
//  18. Ширина правой брови
func ComputeGeometricDescriptor(landmarks []Landmark) GeometricDescriptor {
	if len(landmarks) < 68 {
		return nil
	}

	// Ключевые индексы landmarks (face-api.js 68-point model):
	// Контур лица: 0-16
	// Левая бровь: 17-21
	// Правая бровь: 22-26
	// Нос (спинка): 27-30, крылья: 31-35
	// Левый глаз: 36-41
	// Правый глаз: 42-47
	// Рот (внешний): 48-59, рот (внутренний): 60-67

	lm := landmarks

	// Центры глаз
	leftEyeCenter := center(lm[36], lm[39])
	rightEyeCenter := center(lm[42], lm[45])

	// Нормализующее расстояние — межзрачковое расстояние
	eyeDist := dist(leftEyeCenter, rightEyeCenter)
	if eyeDist < 1e-6 {
		return nil
	}

	// Нормализуем все расстояния относительно межзрачкового
	n := func(d float64) float64 {
		return d / eyeDist
	}

	features := make(GeometricDescriptor, 18)

	// 1. Межзрачковое расстояние (нормализовано = 1.0, эталон)
	features[0] = 1.0

	// 2. Ширина левого глаза
	features[1] = n(dist(lm[36], lm[39]))

	// 3. Ширина правого глаза
	features[2] = n(dist(lm[42], lm[45]))

	// 4. Высота левого глаза
	features[3] = n((dist(lm[37], lm[41]) + dist(lm[38], lm[40])) / 2)

	// 5. Высота правого глаза
	features[4] = n((dist(lm[43], lm[47]) + dist(lm[44], lm[46])) / 2)

	// 6. Длина носа (от переносицы до кончика)
	features[5] = n(dist(lm[27], lm[30]))

	// 7. Ширина носа (крылья)
	features[6] = n(dist(lm[31], lm[35]))

	// 8. Ширина рта
	features[7] = n(dist(lm[48], lm[54]))

	// 9. Высота рта
	features[8] = n(dist(lm[51], lm[57]))

	// 10. Расстояние от кончика носа до верхней губы
	features[9] = n(dist(lm[30], lm[51]))

	// 11. Расстояние от нижней губы до подбородка
	features[10] = n(dist(lm[57], lm[8]))

	// 12. Ширина лица (от уха до уха по скулам)
	features[11] = n(dist(lm[0], lm[16]))

	// 13. Высота лица (лоб — подбородок)
	foreheadCenter := center(lm[19], lm[24]) // между бровями
	features[12] = n(dist(foreheadCenter, lm[8]))

	// 14. Отношение ширины лица к высоте
	if features[12] > 0 {
		features[13] = features[11] / features[12]
	}

	// 15. Расстояние от левого глаза до кончика носа
	features[14] = n(dist(leftEyeCenter, lm[30]))

	// 16. Расстояние от правого глаза до кончика носа
	features[15] = n(dist(rightEyeCenter, lm[30]))

	// 17. Ширина левой брови
	features[16] = n(dist(lm[17], lm[21]))

	// 18. Ширина правой брови
	features[17] = n(dist(lm[22], lm[26]))

	return features
}

// CompareGeometric вычисляет евклидово расстояние между двумя геометрическими дескрипторами.
func CompareGeometric(a, b GeometricDescriptor) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return math.MaxFloat64
	}
	sum := 0.0
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return math.Sqrt(sum)
}

// CompareGeometricCosine вычисляет косинусное расстояние (1 - cosine_similarity).
func CompareGeometricCosine(a, b GeometricDescriptor) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return math.MaxFloat64
	}
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return math.MaxFloat64
	}
	cosSim := dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
	return 1 - cosSim
}

// Вспомогательные функции

func dist(a, b Landmark) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func center(a, b Landmark) Landmark {
	return Landmark{
		X: (a.X + b.X) / 2,
		Y: (a.Y + b.Y) / 2,
	}
}

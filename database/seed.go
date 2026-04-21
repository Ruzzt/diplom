package database

import (
	"face-auth-system/models"
	"log"
	"os"
	"time"
)

func parseDate(s string) *time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return &t
}

func uintPtr(v uint) *uint {
	return &v
}

// Seed заполняет базу тестовыми данными (проекты, сметы, документы)
func Seed() {
	// Проверяем, есть ли уже проекты
	var count int64
	DB.Model(&models.Project{}).Count(&count)
	if count > 0 {
		log.Println("Тестовые данные уже есть, пропускаем заполнение")
		return
	}

	// Находим первого пользователя (админа)
	var user models.User
	if err := DB.First(&user).Error; err != nil {
		log.Println("Нет пользователей для заполнения данных, пропускаем")
		return
	}

	log.Println("Заполнение базы тестовыми данными...")

	// ==================== ПРОЕКТЫ ====================

	projects := []models.Project{
		{
			Name:        "ЖК «Солнечный»",
			Description: "Строительство жилого комплекса на 120 квартир. 3 корпуса, подземный паркинг, детская площадка, благоустройство территории.",
			Status:      "active",
			Address:     "г. Москва, ул. Строителей, 15",
			StartDate:   parseDate("2025-03-01"),
			EndDate:     parseDate("2027-06-30"),
			Budget:      450000000,
			CreatedByID: user.ID,
		},
		{
			Name:        "БЦ «Горизонт»",
			Description: "Бизнес-центр класса B+. 12 этажей, общая площадь 18 000 м². Офисные помещения, конференц-залы, фитнес-центр на первом этаже.",
			Status:      "active",
			Address:     "г. Москва, Ленинский пр-т, 47",
			StartDate:   parseDate("2025-01-15"),
			EndDate:     parseDate("2026-12-01"),
			Budget:      320000000,
			CreatedByID: user.ID,
		},
		{
			Name:        "Школа №214",
			Description: "Капитальный ремонт общеобразовательной школы. Замена кровли, утепление фасада, ремонт спортзала, обновление инженерных сетей.",
			Status:      "planning",
			Address:     "г. Москва, ул. Академика Королёва, 8",
			StartDate:   parseDate("2026-06-01"),
			EndDate:     parseDate("2026-12-31"),
			Budget:      85000000,
			CreatedByID: user.ID,
		},
		{
			Name:        "Складской комплекс «Логистик»",
			Description: "Строительство складского комплекса категории А. 2 корпуса по 5000 м², офисное здание, КПП, ограждение территории.",
			Status:      "active",
			Address:     "Московская обл., г. Подольск, промзона Северная",
			StartDate:   parseDate("2025-06-01"),
			EndDate:     parseDate("2026-09-30"),
			Budget:      210000000,
			CreatedByID: user.ID,
		},
		{
			Name:        "Детский сад «Ромашка»",
			Description: "Строительство детского сада на 150 мест. 2 этажа, бассейн, игровые площадки, кухня-столовая.",
			Status:      "completed",
			Address:     "г. Москва, ул. Цветочная, 3",
			StartDate:   parseDate("2024-04-01"),
			EndDate:     parseDate("2025-09-15"),
			Budget:      120000000,
			CreatedByID: user.ID,
		},
		{
			Name:        "Реконструкция моста через р. Сетунь",
			Description: "Капитальная реконструкция автомобильного моста. Усиление опор, замена пролётного строения, обновление дорожного покрытия.",
			Status:      "suspended",
			Address:     "г. Москва, Аминьевское шоссе",
			StartDate:   parseDate("2025-04-01"),
			EndDate:     parseDate("2026-03-31"),
			Budget:      175000000,
			CreatedByID: user.ID,
		},
		{
			Name:        "ТЦ «Меридиан»",
			Description: "Торговый центр площадью 25 000 м². Три этажа, фуд-корт, кинотеатр, подземная парковка на 300 мест.",
			Status:      "planning",
			Address:     "г. Москва, Варшавское шоссе, 128",
			StartDate:   parseDate("2026-09-01"),
			EndDate:     parseDate("2028-06-30"),
			Budget:      580000000,
			CreatedByID: user.ID,
		},
		{
			Name:        "Поликлиника №7 (пристройка)",
			Description: "Строительство пристройки к действующей поликлинике. Дневной стационар, диагностический блок, лифт для МГН.",
			Status:      "active",
			Address:     "г. Москва, Бульвар Рокоссовского, 22",
			StartDate:   parseDate("2025-08-01"),
			EndDate:     parseDate("2026-07-31"),
			Budget:      95000000,
			CreatedByID: user.ID,
		},
	}

	for i := range projects {
		if err := DB.Create(&projects[i]).Error; err != nil {
			log.Printf("Ошибка создания проекта: %v", err)
		}
	}

	// ==================== СМЕТЫ ====================

	estimates := []struct {
		ProjectIndex int
		Name         string
		Items        []models.EstimateItem
	}{
		{
			ProjectIndex: 0, // ЖК Солнечный
			Name:         "Смета на фундаментные работы",
			Items: []models.EstimateItem{
				{Name: "Разработка грунта экскаватором", Unit: "м³", Quantity: 2500, UnitPrice: 850, TotalPrice: 2125000},
				{Name: "Устройство буронабивных свай", Unit: "шт", Quantity: 180, UnitPrice: 45000, TotalPrice: 8100000},
				{Name: "Бетон М300 (поставка и укладка)", Unit: "м³", Quantity: 1200, UnitPrice: 6500, TotalPrice: 7800000},
				{Name: "Арматура A500С", Unit: "т", Quantity: 85, UnitPrice: 62000, TotalPrice: 5270000},
				{Name: "Гидроизоляция фундамента", Unit: "м²", Quantity: 3200, UnitPrice: 1200, TotalPrice: 3840000},
				{Name: "Обратная засыпка с уплотнением", Unit: "м³", Quantity: 1800, UnitPrice: 450, TotalPrice: 810000},
			},
		},
		{
			ProjectIndex: 0, // ЖК Солнечный
			Name:         "Смета на монолитные работы (корпус 1)",
			Items: []models.EstimateItem{
				{Name: "Опалубка стен и колонн", Unit: "м²", Quantity: 5600, UnitPrice: 1800, TotalPrice: 10080000},
				{Name: "Бетон М350", Unit: "м³", Quantity: 3200, UnitPrice: 7200, TotalPrice: 23040000},
				{Name: "Арматурные каркасы", Unit: "т", Quantity: 210, UnitPrice: 68000, TotalPrice: 14280000},
				{Name: "Монтаж/демонтаж опалубки", Unit: "м²", Quantity: 5600, UnitPrice: 650, TotalPrice: 3640000},
				{Name: "Уход за бетоном", Unit: "м³", Quantity: 3200, UnitPrice: 180, TotalPrice: 576000},
			},
		},
		{
			ProjectIndex: 1, // БЦ Горизонт
			Name:         "Смета на фасадные работы",
			Items: []models.EstimateItem{
				{Name: "Вентилируемый фасад (керамогранит)", Unit: "м²", Quantity: 8500, UnitPrice: 4200, TotalPrice: 35700000},
				{Name: "Утеплитель минераловатный 150мм", Unit: "м²", Quantity: 8500, UnitPrice: 980, TotalPrice: 8330000},
				{Name: "Витражное остекление", Unit: "м²", Quantity: 2400, UnitPrice: 12500, TotalPrice: 30000000},
				{Name: "Монтаж подсистемы", Unit: "м²", Quantity: 8500, UnitPrice: 1500, TotalPrice: 12750000},
				{Name: "Водоотливы и откосы", Unit: "м.п.", Quantity: 650, UnitPrice: 2800, TotalPrice: 1820000},
			},
		},
		{
			ProjectIndex: 1, // БЦ Горизонт
			Name:         "Смета на инженерные сети",
			Items: []models.EstimateItem{
				{Name: "Система вентиляции и кондиционирования", Unit: "компл", Quantity: 1, UnitPrice: 18500000, TotalPrice: 18500000},
				{Name: "Электроснабжение (кабельные линии)", Unit: "м.п.", Quantity: 12000, UnitPrice: 1200, TotalPrice: 14400000},
				{Name: "Система пожаротушения", Unit: "компл", Quantity: 1, UnitPrice: 8200000, TotalPrice: 8200000},
				{Name: "Слаботочные сети (СКС, СКУД, видео)", Unit: "компл", Quantity: 1, UnitPrice: 6500000, TotalPrice: 6500000},
				{Name: "Водоснабжение и канализация", Unit: "компл", Quantity: 1, UnitPrice: 5800000, TotalPrice: 5800000},
				{Name: "Лифтовое оборудование", Unit: "шт", Quantity: 4, UnitPrice: 3200000, TotalPrice: 12800000},
			},
		},
		{
			ProjectIndex: 2, // Школа №214
			Name:         "Смета на ремонт кровли",
			Items: []models.EstimateItem{
				{Name: "Демонтаж старого покрытия", Unit: "м²", Quantity: 3500, UnitPrice: 280, TotalPrice: 980000},
				{Name: "Устройство пароизоляции", Unit: "м²", Quantity: 3500, UnitPrice: 350, TotalPrice: 1225000},
				{Name: "Утеплитель XPS 100мм", Unit: "м²", Quantity: 3500, UnitPrice: 780, TotalPrice: 2730000},
				{Name: "ПВХ-мембрана кровельная", Unit: "м²", Quantity: 3500, UnitPrice: 1450, TotalPrice: 5075000},
				{Name: "Водосточная система", Unit: "м.п.", Quantity: 280, UnitPrice: 3200, TotalPrice: 896000},
			},
		},
		{
			ProjectIndex: 3, // Складской комплекс
			Name:         "Смета на металлоконструкции (корпус 1)",
			Items: []models.EstimateItem{
				{Name: "Колонны стальные HEB 300", Unit: "т", Quantity: 45, UnitPrice: 95000, TotalPrice: 4275000},
				{Name: "Фермы покрытия L=24м", Unit: "т", Quantity: 62, UnitPrice: 110000, TotalPrice: 6820000},
				{Name: "Прогоны и связи", Unit: "т", Quantity: 28, UnitPrice: 85000, TotalPrice: 2380000},
				{Name: "Профнастил кровельный Н75", Unit: "м²", Quantity: 5200, UnitPrice: 1100, TotalPrice: 5720000},
				{Name: "Сэндвич-панели стеновые 120мм", Unit: "м²", Quantity: 4800, UnitPrice: 3200, TotalPrice: 15360000},
				{Name: "Монтаж металлоконструкций", Unit: "т", Quantity: 135, UnitPrice: 28000, TotalPrice: 3780000},
			},
		},
		{
			ProjectIndex: 4, // Детский сад
			Name:         "Смета на отделочные работы",
			Items: []models.EstimateItem{
				{Name: "Штукатурка стен", Unit: "м²", Quantity: 6800, UnitPrice: 680, TotalPrice: 4624000},
				{Name: "Покраска стен и потолков", Unit: "м²", Quantity: 9200, UnitPrice: 380, TotalPrice: 3496000},
				{Name: "Керамическая плитка (полы, санузлы)", Unit: "м²", Quantity: 1800, UnitPrice: 2200, TotalPrice: 3960000},
				{Name: "Линолеум коммерческий", Unit: "м²", Quantity: 2400, UnitPrice: 1500, TotalPrice: 3600000},
				{Name: "Подвесные потолки Armstrong", Unit: "м²", Quantity: 3200, UnitPrice: 850, TotalPrice: 2720000},
				{Name: "Двери межкомнатные", Unit: "шт", Quantity: 85, UnitPrice: 12000, TotalPrice: 1020000},
			},
		},
		{
			ProjectIndex: 7, // Поликлиника
			Name:         "Смета на общестроительные работы",
			Items: []models.EstimateItem{
				{Name: "Кладка стен из газобетона", Unit: "м³", Quantity: 420, UnitPrice: 5500, TotalPrice: 2310000},
				{Name: "Монолитные перекрытия", Unit: "м²", Quantity: 1200, UnitPrice: 4800, TotalPrice: 5760000},
				{Name: "Устройство кровли (плоская)", Unit: "м²", Quantity: 600, UnitPrice: 3200, TotalPrice: 1920000},
				{Name: "Окна ПВХ двухкамерные", Unit: "м²", Quantity: 380, UnitPrice: 8500, TotalPrice: 3230000},
				{Name: "Устройство лифтовой шахты", Unit: "компл", Quantity: 1, UnitPrice: 2800000, TotalPrice: 2800000},
			},
		},
	}

	for _, est := range estimates {
		var totalAmount float64
		for _, item := range est.Items {
			totalAmount += item.TotalPrice
		}

		estimate := models.Estimate{
			ProjectID:   projects[est.ProjectIndex].ID,
			Name:        est.Name,
			TotalAmount: totalAmount,
			Items:       est.Items,
			CreatedByID: user.ID,
		}

		if err := DB.Create(&estimate).Error; err != nil {
			log.Printf("Ошибка создания сметы: %v", err)
		}
	}

	// ==================== ДОКУМЕНТЫ ====================

	// Создаём папку для файлов
	os.MkdirAll("uploads", 0755)

	documents := []struct {
		ProjectIndex *int
		Name         string
		DocType      string
		FileSize     int64
	}{
		// Документы проектов
		{intPtr(0), "Разрешение на строительство ЖК Солнечный", "permit", 2450000},
		{intPtr(0), "Генеральный план ЖК Солнечный", "blueprint", 8900000},
		{intPtr(0), "Договор генподряда №127-СМР", "contract", 1850000},
		{intPtr(1), "Архитектурный проект БЦ Горизонт", "blueprint", 12400000},
		{intPtr(1), "Разрешение на строительство БЦ Горизонт", "permit", 3200000},
		{intPtr(1), "Договор аренды земельного участка", "contract", 980000},
		{intPtr(2), "Проект капитального ремонта школы", "blueprint", 5600000},
		{intPtr(2), "Заключение экспертизы проектной документации", "report", 1200000},
		{intPtr(3), "Рабочая документация (раздел КМ)", "blueprint", 7300000},
		{intPtr(3), "Договор поставки металлоконструкций", "contract", 1400000},
		{intPtr(4), "Акт ввода в эксплуатацию", "report", 890000},
		{intPtr(4), "Исполнительная документация (полный комплект)", "other", 15600000},
		{intPtr(5), "Отчёт об обследовании моста", "report", 4500000},
		{intPtr(5), "Проект реконструкции моста", "blueprint", 9800000},
		{intPtr(7), "Медицинское техзадание", "other", 760000},
		{intPtr(7), "Договор подряда №89-ПР", "contract", 2100000},
		// Корпоративные документы (без проекта)
		{nil, "Устав организации", "other", 540000},
		{nil, "Лицензия на строительную деятельность", "permit", 1200000},
		{nil, "СРО — допуск к работам", "permit", 890000},
		{nil, "Шаблон договора подряда", "contract", 320000},
		{nil, "Регламент охраны труда", "report", 1800000},
	}

	for _, doc := range documents {
		// Создаём пустой файл-заглушку
		filePath := "uploads/" + doc.Name + ".pdf"
		os.WriteFile(filePath, []byte("%PDF-1.4 placeholder"), 0644)

		var projectID *uint
		if doc.ProjectIndex != nil {
			projectID = uintPtr(projects[*doc.ProjectIndex].ID)
		}

		document := models.Document{
			ProjectID:    projectID,
			Name:         doc.Name,
			FilePath:     filePath,
			DocType:      doc.DocType,
			FileSize:     doc.FileSize,
			UploadedByID: user.ID,
		}

		if err := DB.Create(&document).Error; err != nil {
			log.Printf("Ошибка создания документа: %v", err)
		}
	}

	log.Println("Тестовые данные успешно добавлены!")
	log.Printf("  Проектов: %d", len(projects))
	log.Printf("  Смет: %d", len(estimates))
	log.Printf("  Документов: %d", len(documents))
}

func intPtr(v int) *int {
	return &v
}

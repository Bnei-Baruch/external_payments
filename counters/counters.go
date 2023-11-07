package counters

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"

	"external_payments/db"
	"external_payments/types"
	"external_payments/utils"
)

var translations = map[string]map[string]string{
	"en": {
		"contributors":      "contributors",
		"stat_contributors": "contributors",
		"of":                "of",
		"goal":              "goal",
		"contribute":        "contribute",
		"statistics":        "Statistics",
		"sum":               "Sum",
		"country":           "Country",
		"all countries":     "All countries »",
		"to":                "to",
	},
	"es": {
		"contributors":      "donantes",
		"stat_contributors": "Donantes",
		"of":                "de",
		"goal":              "suma",
		"contribute":        "efectue el pago",
		"statistics":        "Estadística",
		"sum":               "Suma",
		"country":           "Pais",
		"all countries":     "All countries »",
		"to":                "to",
	},
	"he": {
		"contributors":      "תורמים",
		"stat_contributors": "תורמים",
		"of":                "מתוך סכום של",
		"goal":              "",
		"contribute":        "בצע תשלום",
		"statistics":        "סטטיסטיקה",
		"sum":               "סכום",
		"country":           "מדינה",
		"all countries":     "כל המדינות »",
		"to":                "עד",
	},
	"ru": {
		"contributors":      "доноров",
		"stat_contributors": "Доноры",
		"of":                "от цели в",
		"goal":              "",
		"contribute":        "оплата",
		"statistics":        "Cтатистика",
		"sum":               "Сумма",
		"country":           "Страна",
		"all countries":     "Все страны »",
		"to":                "до",
	},
}

func Counter(c *gin.Context) {
	projectName := c.Param("project_name")
	if projectName == "" {
		c.HTML(http.StatusBadRequest, "404.html", gin.H{"error": "projectName is required"})
		return
	}
	bgcolor := "white"
	if bg, ok := c.GetQuery("bgcolor"); ok {
		bgcolor = bg
	}
	language := c.Param("language")
	if language == "" {
		language = "en"
	}
	words := translations["en"]
	if tr, has := translations[language]; has {
		words = tr
	}
	project, err := db.GetProject(projectName)
	if err != nil {
		project.Target = 1_000_000
		project.StartDate = "2023-08-01 00:00:00"
	}
	donors, amount := CalculateTotals(project.Name, project.StartDate)
	percent := fmt.Sprint(math.Round(amount*100/project.Target*100) / 100)
	var urls map[string]string
	json.Unmarshal([]byte(project.Url), &urls)
	url := "https://neworg.kbb1.com/en/node/1050"
	fmt.Printf("Default url %s\n", url)
	if value, ok := urls[language]; ok {
		url = value
		fmt.Printf("language %s url %s\n", language, url)
	} else if value, ok := urls["en"]; ok {
		url = value
		fmt.Printf("language en url %s\n", url)
	}
	fmt.Printf("url %s\n", url)
	c.HTML(http.StatusOK, "counter.tmpl", gin.H{
		"language":     language,
		"bgcolor":      bgcolor,
		"donors":       donors,
		"amount":       amount,
		"target":       project.Target,
		"percent":      percent,
		"contributors": words["contributors"],
		"of":           words["of"],
		"goal":         words["goal"],
		"contribute":   words["contribute"],
		"url":          url,
	})
}

func CalculateTotals(projectName, startDate string) (donors float64, total float64) {
	totalUSD, err := db.GetProjectTotals(projectName, "USD", startDate)
	if err != nil {
		totalUSD = types.ProjectTotals{
			Contributors: 0,
			Total:        0,
		}
	}
	totalEUR, err := db.GetProjectTotals(projectName, "EUR", startDate)
	if err != nil {
		totalEUR = types.ProjectTotals{
			Contributors: 0,
			Total:        0,
		}
	}
	totalILS, err := db.GetProjectTotals(projectName, "ILS", startDate)
	if err != nil {
		totalILS = types.ProjectTotals{
			Contributors: 0,
			Total:        0,
		}
	}
	donors = totalILS.Contributors + totalEUR.Contributors + totalUSD.Contributors
	total = totalUSD.Total + totalEUR.Total*1.1 + totalILS.Total/4.13
	return
}

func Statistics(c *gin.Context) {
	projectName := c.Param("project_name")
	if projectName == "" {
		c.HTML(http.StatusBadRequest, "404.html", gin.H{"error": "projectName is required"})
		return
	}
	bgcolor := "white"
	if bg, ok := c.GetQuery("bgcolor"); ok {
		bgcolor = bg
	}
	language := c.Param("language")
	if language == "" {
		language = "en"
	}
	words := translations["en"]
	if tr, has := translations[language]; has {
		words = tr
	}
	project, err := db.GetProject(projectName)
	if err != nil {
		project.Target = 1_000_000
		project.StartDate = "2023-08-01 00:00:00"
	}
	ranges, err := db.GetProjectRanges(project.Name, project.StartDate)
	if err != nil {
		utils.LogMessage(fmt.Sprint("======> GetProjectRanges", err.Error()))
		ranges = []types.ProjectRange{
			{Start: 1, Finish: 9, Contributors: 12},
			{Start: 10, Finish: 99, Contributors: 12},
			{Start: 100, Finish: 999, Contributors: 12},
			{Start: 1000, Finish: 4999, Contributors: 12},
			{Start: 5000, Finish: 9999, Contributors: 12},
			{Start: 10000, Finish: 99999, Contributors: 12},
			{Start: 100000, Finish: 999999, Contributors: 12},
			{Start: 1000000, Finish: 9999999, Contributors: 12},
		}
	}
	byCountry, err := db.GetProjectByCountry(project.Name, project.StartDate)
	if err != nil {
		utils.LogMessage(fmt.Sprint("======> GetProjectByCountry", err.Error()))
		byCountry = []types.ProjectByCountry{}
	}
	c.HTML(http.StatusOK, "statistics.tmpl", gin.H{
		"language":      language,
		"bgcolor":       bgcolor,
		"statistics":    words["statistics"],
		"contributors":  words["stat_contributors"],
		"sum":           words["sum"],
		"country":       words["country"],
		"all_countries": words["all countries"],
		"to":            words["to"],
		"ranges":        ranges,
		"countries":     byCountry,
	})
}

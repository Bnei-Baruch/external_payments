package hmarket

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"external_payments/db"
	"external_payments/types"
)

func Export(c *gin.Context) {
	rows, err := db.GetHMarketExportData()
	if err != nil {
		log.Printf("[hmarket/export] query error: %v", err)
		c.JSON(500, gin.H{"error": "db error"})
		return
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"

	headers := []string{
		"ID", "First Name", "Last Name", "Phone", "Uniq Phone", "Email",
		"Company", "City", "Country", "Subscribed", "Blacklisted",
		"Source", "Product Name", "Product ID", "SKU", "Created At", "Circle",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	for i, row := range rows {
		r := i + 2
		values := []any{
			row.UserID, row.FirstName, row.LastName, row.Phone, row.UniqPhone, row.Email,
			row.Company, row.City, row.Country, row.Subscribed, row.Blacklisted,
			row.Source, row.Name, row.ProductID, row.SKU, row.CreatedAt, row.Circle,
		}
		for j, v := range values {
			cell, _ := excelize.CoordinatesToCellName(j+1, r)
			f.SetCellValue(sheet, cell, v)
		}
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=hmarket_export.xlsx")
	if err := f.Write(c.Writer); err != nil {
		log.Printf("[hmarket/export] write error: %v", err)
	}
}

func Audiences(c *gin.Context) {
	rows, err := db.GetHMarketAudiencesByMonth()
	if err != nil {
		log.Printf("[hmarket/audiences] query error: %v", err)
		c.JSON(500, gin.H{"error": "db error"})
		return
	}

	monthSet := map[string]struct{}{}
	sourceSet := map[string]struct{}{}
	newCounts := map[string]map[string]int64{}

	for _, r := range rows {
		monthSet[r.Month] = struct{}{}
		sourceSet[r.Source] = struct{}{}
		if newCounts[r.Source] == nil {
			newCounts[r.Source] = map[string]int64{}
		}
		newCounts[r.Source][r.Month] += r.Count
	}

	months := make([]string, 0, len(monthSet))
	for m := range monthSet {
		months = append(months, m)
	}
	sort.Strings(months)

	sources := make([]string, 0, len(sourceSet))
	for s := range sourceSet {
		sources = append(sources, s)
	}
	sort.Strings(sources)

	// Cumulative totals per source, indexed by month position
	cumul := map[string][]int64{}
	for _, src := range sources {
		cumul[src] = make([]int64, len(months))
		var running int64
		for i, m := range months {
			running += newCounts[src][m]
			cumul[src][i] = running
		}
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"

	// Header: "Source" | month | "%" | month | "%" | ...
	f.SetCellValue(sheet, "A1", "Source")
	for i, m := range months {
		col := i*2 + 2
		monthCell, _ := excelize.CoordinatesToCellName(col, 1)
		f.SetCellValue(sheet, monthCell, m)
		pctCell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheet, pctCell, "%")
	}

	row := 2
	for _, src := range sources {
		srcCell, _ := excelize.CoordinatesToCellName(1, row)
		f.SetCellValue(sheet, srcCell, src)
		for i := range months {
			col := i*2 + 2
			countCell, _ := excelize.CoordinatesToCellName(col, row)
			f.SetCellValue(sheet, countCell, cumul[src][i])
			if i > 0 && cumul[src][i-1] > 0 {
				pct := math.Round(float64(cumul[src][i]-cumul[src][i-1])/float64(cumul[src][i-1])*1000) / 10
				pctCell, _ := excelize.CoordinatesToCellName(col+1, row)
				f.SetCellValue(sheet, pctCell, pct)
			}
		}
		row++
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=audiences.xlsx")
	if err := f.Write(c.Writer); err != nil {
		log.Printf("[hmarket/audiences] write error: %v", err)
	}
}

func Status(c *gin.Context) {
	users, activities, err := db.GetHMarketStatus()
	if err != nil {
		log.Printf("[hmarket/status] query error: %v", err)
		c.JSON(500, gin.H{"error": "db error"})
		return
	}
	c.JSON(200, gin.H{"users": users, "activities": activities})
}

func SubscriptionStatus(c *gin.Context) {
	users, err := db.GetHMarketUsers()
	if err != nil {
		log.Printf("[hmarket/subscription-status] users query error: %v", err)
		c.JSON(500, gin.H{"error": "db error"})
		return
	}

	history, err := db.GetHMarketSubHistory()
	if err != nil {
		log.Printf("[hmarket/subscription-status] history query error: %v", err)
		c.JSON(500, gin.H{"error": "db error"})
		return
	}

	byUser := make(map[int64][]types.HMarketSubHistoryRecord)
	for _, h := range history {
		byUser[h.UserID] = append(byUser[h.UserID], h)
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"

	headers := []string{"ID", "First Name", "Last Name", "Phone", "Email", "Subscribed", "Blacklisted", "History"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	wrapStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{WrapText: true},
	})

	for ri, u := range users {
		r := ri + 2
		phone := ""
		if u.Phone != nil {
			phone = *u.Phone
		}

		var lines []string
		for _, ch := range byUser[u.ID] {
			status := "false"
			if ch.Status {
				status = "true"
			}
			lines = append(lines, fmt.Sprintf("%s | %s | %s | %s", ch.CreatedAt, ch.ChangeType, status, ch.Description))
		}

		values := []any{
			u.ID, u.FirstName, u.LastName, phone, u.Email,
			u.Subscribed, u.Blacklisted,
			strings.Join(lines, "\n"),
		}
		for j, v := range values {
			cell, _ := excelize.CoordinatesToCellName(j+1, r)
			f.SetCellValue(sheet, cell, v)
		}

		historyCell, _ := excelize.CoordinatesToCellName(8, r)
		f.SetCellStyle(sheet, historyCell, historyCell, wrapStyle)
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=hmarket_subscription_status.xlsx")
	if err := f.Write(c.Writer); err != nil {
		log.Printf("[hmarket/subscription-status] write error: %v", err)
	}
}

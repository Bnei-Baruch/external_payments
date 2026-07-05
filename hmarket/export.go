package hmarket

import (
	"fmt"
	"log"
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

	// Collect months and sources; store new-user counts per (source, month, subscribed)
	monthSet := map[string]struct{}{}
	sourceSet := map[string]struct{}{}
	newCounts := map[string]map[string]map[bool]int64{}

	for _, r := range rows {
		monthSet[r.Month] = struct{}{}
		sourceSet[r.Source] = struct{}{}
		if newCounts[r.Source] == nil {
			newCounts[r.Source] = map[string]map[bool]int64{}
		}
		if newCounts[r.Source][r.Month] == nil {
			newCounts[r.Source][r.Month] = map[bool]int64{}
		}
		newCounts[r.Source][r.Month][r.Subscribed] += r.Count
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

	// Cumulate running totals per (source, subscribed) across months
	type cumKey struct {
		source     string
		subscribed bool
	}
	running := map[cumKey]int64{}
	cumul := map[string]map[string]map[bool]int64{}
	for _, src := range sources {
		cumul[src] = map[string]map[bool]int64{}
		for _, m := range months {
			cumul[src][m] = map[bool]int64{}
			for _, sub := range []bool{false, true} {
				k := cumKey{src, sub}
				running[k] += newCounts[src][m][sub]
				cumul[src][m][sub] = running[k]
			}
		}
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"

	// Header row: "Source" then one column per month
	f.SetCellValue(sheet, "A1", "Source")
	for i, m := range months {
		cell, _ := excelize.CoordinatesToCellName(i+2, 1)
		f.SetCellValue(sheet, cell, m)
	}

	row := 2
	for _, src := range sources {
		for _, info := range []struct {
			label string
			sub   bool
			total bool
		}{
			{src + " - Interested", false, false},
			{src + " - Bnei Baruch", true, false},
			{src + " - Total", false, true},
		} {
			cell, _ := excelize.CoordinatesToCellName(1, row)
			f.SetCellValue(sheet, cell, info.label)
			for i, m := range months {
				cell, _ := excelize.CoordinatesToCellName(i+2, row)
				var v int64
				if info.total {
					v = cumul[src][m][false] + cumul[src][m][true]
				} else {
					v = cumul[src][m][info.sub]
				}
				f.SetCellValue(sheet, cell, v)
			}
			row++
		}
		row++ // blank separator between sources
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

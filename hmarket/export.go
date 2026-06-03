package hmarket

import (
	"fmt"
	"log"
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
		"Source", "Product Name", "Product ID", "SKU", "Created At",
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
			row.Source, row.Name, row.ProductID, row.SKU, row.CreatedAt,
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
			lines = append(lines, fmt.Sprintf("%s | %s | %s", ch.CreatedAt, status, ch.Description))
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

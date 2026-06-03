package hmarket

import (
	"fmt"
	"log"

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
		"ID", "First Name", "Last Name", "Phone", "Email",
		"Company", "City", "Country",
		"Source", "Product Name", "Product ID", "SKU", "Created At",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	for i, row := range rows {
		r := i + 2
		values := []any{
			row.UserID, row.FirstName, row.LastName, row.Phone, row.Email,
			row.Company, row.City, row.Country,
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
	maxChanges := 0
	for _, h := range history {
		byUser[h.UserID] = append(byUser[h.UserID], h)
		if len(byUser[h.UserID]) > maxChanges {
			maxChanges = len(byUser[h.UserID])
		}
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"

	headers := []string{"ID", "First Name", "Last Name", "Phone", "Email", "Subscribed", "Blacklisted"}
	for i := 1; i <= maxChanges; i++ {
		headers = append(headers,
			fmt.Sprintf("Change %d Status", i),
			fmt.Sprintf("Change %d Date", i),
			fmt.Sprintf("Change %d Description", i),
		)
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	for ri, u := range users {
		r := ri + 2
		phone := ""
		if u.Phone != nil {
			phone = *u.Phone
		}
		values := []any{u.ID, u.FirstName, u.LastName, phone, u.Email, u.Subscribed, u.Blacklisted}
		for _, ch := range byUser[u.ID] {
			values = append(values, ch.Status, ch.CreatedAt, ch.Description)
		}
		for j, v := range values {
			cell, _ := excelize.CoordinatesToCellName(j+1, r)
			f.SetCellValue(sheet, cell, v)
		}
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=hmarket_subscription_status.xlsx")
	if err := f.Write(c.Writer); err != nil {
		log.Printf("[hmarket/subscription-status] write error: %v", err)
	}
}

package api

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

func ParsePagination(c *gin.Context) (page int, pageSize int, err error) {
	page = 1
	pageSize = defaultPageSize

	if pageParam := c.Query("page"); pageParam != "" {
		page, err = strconv.Atoi(pageParam)
		if err != nil || page < 1 {
			return 0, 0, errors.New("page must be an integer greater than or equal to 1")
		}
	}

	if pageSizeParam := c.Query("page_size"); pageSizeParam != "" {
		pageSize, err = strconv.Atoi(pageSizeParam)
		if err != nil || pageSize < 1 || pageSize > maxPageSize {
			return 0, 0, errors.New("page_size must be an integer between 1 and 100")
		}
	}

	return page, pageSize, nil
}

func PaginationMeta(page int, pageSize int, total int64) Pagination {
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages == 0 {
		totalPages = 1
	}

	return Pagination{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}
}

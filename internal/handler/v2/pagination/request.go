package pagination

import (
	"errors"
	"fmt"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"strconv"
)

func GetPagination(ctx *goskell.Context, defaultPageSize, maxPageSize uint) (internal.PaginationParams, error) {
	paginationParams := internal.PaginationParams{Page: 1, Size: defaultPageSize}
	if page := ctx.QueryMap("page"); len(page) != 0 {
		if pageRaw, ok := page["number"]; ok {
			number, err := strconv.Atoi(pageRaw)
			if err != nil || number <= 0 {
				return paginationParams, errors.New("invalid input for page. Must be a number and can't be 0 or lower")
			}
			paginationParams.Page = uint(number)
		}
		if sizeRaw, ok := page["size"]; ok {
			size, err := strconv.ParseUint(sizeRaw, 10, 8)
			if err != nil || size <= 0 {
				return paginationParams, errors.New("invalid input for size. Must be a number and can't be 0 or lower")
			}
			if uint(size) > maxPageSize {
				return paginationParams, fmt.Errorf("invalid page size. The max page size is: %d", maxPageSize)
			}
			paginationParams.Size = uint(size)
		}
	}

	return paginationParams, nil
}

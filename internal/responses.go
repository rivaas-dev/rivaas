package internal

import (
	"fmt"
	"math"
	"net/url"

	"github.com/google/jsonapi"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
)

type PaginationParams struct {
	Size uint
	Page uint
}

// WriteJSONAPIResponse writes a json api response.
func WriteJSONAPIResponse(ctx *goskell.Context, body any, statusCode int, headers map[string]string) {
	for key, value := range headers {
		ctx.Header(key, value)
	}
	ctx.Header("Content-Type", jsonapi.MediaType)
	ctx.JSON(statusCode, body)
}

// WriteResponse writes a byte json api response.
func WriteResponse(ctx *goskell.Context, body []byte, statusCode int, headers map[string]string) {
	for key, value := range headers {
		ctx.Header(key, value)
	}
	ctx.Data(statusCode, jsonapi.MediaType, body)
}

// GeneratePageLinks generates page links.
func GeneratePageLinks(ctx *goskell.Context, paginationParams *PaginationParams, totalResult uint) (*jsonapi.Links, error) {
	// Get the previous path
	base, err := url.Parse(ctx.Request.URL.String())
	if err != nil {
		return nil, err
	}
	queries := base.Query()

	// Cleanup the 'page' parameters.
	queries.Del("page[size]")
	queries.Del("page[number]")

	// Generate base url.
	base.RawQuery = queries.Encode()
	baseurl, err := url.QueryUnescape(fmt.Sprintf("%s?%s", base.Path, base.RawQuery))
	if err != nil {
		return nil, err
	}

	fmtURL := "%s&%s=%d&%s=%d"
	if base.RawQuery == "" { // if there are no query parameters, skip the '&'
		fmtURL = "%s%s=%d&%s=%d"
	}
	// Pages.
	links := jsonapi.Links{
		"self": fmt.Sprintf(
			fmtURL,
			baseurl,
			jsonapi.QueryParamPageSize,
			paginationParams.Size,
			jsonapi.QueryParamPageNumber,
			paginationParams.Page,
		),
	}
	totalPage := uint(math.Ceil(float64(totalResult) / float64(paginationParams.Size)))

	// Next page.
	if totalPage > paginationParams.Page {
		links[jsonapi.KeyNextPage] = fmt.Sprintf(
			fmtURL,
			baseurl,
			jsonapi.QueryParamPageSize,
			paginationParams.Size,
			jsonapi.QueryParamPageNumber,
			paginationParams.Page+1,
		)
		links[jsonapi.KeyLastPage] = fmt.Sprintf(
			fmtURL,
			baseurl,
			jsonapi.QueryParamPageSize,
			paginationParams.Size,
			jsonapi.QueryParamPageNumber,
			totalPage,
		)
	}

	// Previous page.
	if paginationParams.Page > 1 && paginationParams.Page <= totalPage {
		links[jsonapi.KeyPreviousPage] = fmt.Sprintf(
			fmtURL,
			baseurl,
			jsonapi.QueryParamPageSize,
			paginationParams.Size,
			jsonapi.QueryParamPageNumber,
			paginationParams.Page-1,
		)
		links[jsonapi.KeyFirstPage] = fmt.Sprintf(
			fmtURL,
			baseurl,
			jsonapi.QueryParamPageSize,
			paginationParams.Size,
			jsonapi.QueryParamPageNumber,
			1,
		)
	}

	return &links, nil
}

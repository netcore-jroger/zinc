package v2

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"github.com/rs/zerolog/log"

	"github.com/zinclabs/zinc/pkg/core"
	"github.com/zinclabs/zinc/pkg/errors"
	meta "github.com/zinclabs/zinc/pkg/meta/v2"
)

// SearchIndex searches the index for the given http request from end user
func SearchIndex(c *gin.Context) {
	indexName := c.Param("target")

	query := new(meta.ZincQuery)
	if err := c.BindJSON(query); err != nil {
		log.Printf("handlers.v2.SearchIndex: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := searchIndex(indexName, query)
	if err != nil {
		handleError(c, err)
		return
	}

	eventData := make(map[string]interface{})
	eventData["search_type"] = "query_dsl"
	eventData["search_index_storage"] = "disk"
	eventData["search_index_size_in_mb"] = 0.0
	eventData["time_taken_to_search_in_ms"] = resp.Took
	eventData["aggregations_count"] = len(query.Aggregations)
	core.Telemetry.Event("search", eventData)

	c.JSON(http.StatusOK, resp)
}

// MultipleSearch like bulk searches
func MultipleSearch(c *gin.Context) {
	defaultIndexName := c.Param("target")

	responses := make([]interface{}, 0)

	// Prepare to read the entire raw text of the body
	scanner := bufio.NewScanner(c.Request.Body)
	defer c.Request.Body.Close()

	// Set 1 MB max per line. docs at - https://pkg.go.dev/bufio#pkg-constants
	// This is the max size of a line in a file that we will process
	const maxCapacityPerLine = 1024 * 1024
	buf := make([]byte, maxCapacityPerLine)
	scanner.Buffer(buf, maxCapacityPerLine)

	indexName := ""
	nextLineIsData := false

	var doc map[string]interface{}
	var err error
	for scanner.Scan() { // Read each line
		if nextLineIsData {
			nextLineIsData = false
			var query *meta.ZincQuery
			if err = json.Unmarshal(scanner.Bytes(), &query); err != nil {
				log.Error().Msgf("handlers.v2.MultipleSearch: json.Unmarshal: err %s", err.Error())
				responses = append(responses, &meta.SearchResponse{Error: err.Error()})
				continue
			}
			// search query
			resp, err := searchIndex(indexName, query)
			if err != nil {
				log.Error().Msgf("handlers.v2.MultipleSearch: searchIndex: err %s", err.Error())
				responses = append(responses, &meta.SearchResponse{Error: err.Error()})
			} else {
				responses = append(responses, resp)
			}
		} else {
			nextLineIsData = true
			if err = json.Unmarshal(scanner.Bytes(), &doc); err != nil {
				log.Error().Msgf("handlers.v2.MultipleSearch: json.Unmarshal: err %s", err.Error())
				continue
			}
			if v, ok := doc["index"]; ok {
				indexName = v.(string)
			} else {
				indexName = defaultIndexName
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"responses": responses})
}

func searchIndex(indexName string, query *meta.ZincQuery) (*meta.SearchResponse, error) {
	var err error
	var resp *meta.SearchResponse
	if indexName == "" || strings.HasSuffix(indexName, "*") {
		resp, err = core.MultiSearchV2(indexName, query)
	} else {
		index, exists := core.GetIndex(indexName)
		if !exists {
			return nil, fmt.Errorf("index %s does not exists", indexName)
		}
		resp, err = index.SearchV2(query)
	}
	return resp, err
}

func handleError(c *gin.Context, err error) {
	if err != nil {
		switch v := err.(type) {
		case *errors.Error:
			c.JSON(http.StatusBadRequest, gin.H{"error": v})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": v.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

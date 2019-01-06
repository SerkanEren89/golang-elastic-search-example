package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/olivere/elastic"
	"github.com/teris-io/shortid"
)

const (
	elasticIndexName = "documents"
	elasticTypeName  = "document"
)

type Document struct {
	ID          string    `json:"id"`
	ISBN        string    `json:"isbn"`
	Title       string    `json:"title"`
	SubTitle    string    `json:"subtitle"`
	Author      string    `json:"author"`
	Published   time.Time `json:"published"`
	Publisher   string    `json:"publisher"`
	Pages       int       `json:"pages"`
	Description string    `json:"description"`
	Website     string    `json:"website"`
	CreatedAt   time.Time `json:"created_at"`
}

type DocumentRequest struct {
	ISBN        string    `json:"isbn"`
	Title       string    `json:"title"`
	SubTitle    string    `json:"subtitle"`
	Author      string    `json:"author"`
	Published   time.Time `json:"published"`
	Publisher   string    `json:"publisher"`
	Pages       int       `json:"pages"`
	Description string    `json:"description"`
	Website     string    `json:"website"`
}

type DocumentResponse struct {
	ISBN        string    `json:"isbn"`
	Title       string    `json:"title"`
	SubTitle    string    `json:"subtitle"`
	Author      string    `json:"author"`
	Published   time.Time `json:"published"`
	Publisher   string    `json:"publisher"`
	Pages       int       `json:"pages"`
	Description string    `json:"description"`
	Website     string    `json:"website"`
	CreatedAt   time.Time `json:"created_at"`
}

type SearchResponse struct {
	Time      string             `json:"time"`
	Hits      string             `json:"hits"`
	Documents []DocumentResponse `json:"documents"`
}

var elasticClient *elastic.Client

func main() {
	var err error
	// Create Elastic client and wait for to be ready
	for {
		elasticClient, err = elastic.NewClient(
			elastic.SetURL("http://elasticsearch:9200"),
			elastic.SetSniff(false),
		)
		if err != nil {
			log.Println(err)
			// try until elastic search client is ready
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
	// Start HTTP server
	r := gin.Default()
	r.POST("/documents", createDocumentsEndpoint)
	r.GET("/documents", searchEndpoint)
	if err = r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

func createDocumentsEndpoint(c *gin.Context) {
	// Parse request
	var docs []DocumentRequest
	if err := c.BindJSON(&docs); err != nil {
		errorResponse(c, http.StatusBadRequest, "Malformed request body")
		return
	}
	// Insert documents in bulk
	bulk := elasticClient.
		Bulk().
		Index(elasticIndexName).
		Type(elasticTypeName)
	for _, d := range docs {
		doc := Document{
			ID:          shortid.MustGenerate(),
			ISBN:        d.ISBN,
			Title:       d.Title,
			SubTitle:    d.SubTitle,
			Author:      d.Author,
			Published:   d.Published,
			Publisher:   d.Publisher,
			Pages:       d.Pages,
			Description: d.Description,
			CreatedAt:   time.Now().UTC(),
		}
		bulk.Add(elastic.NewBulkIndexRequest().Id(doc.ID).Doc(doc))
	}
	if _, err := bulk.Do(c.Request.Context()); err != nil {
		log.Println(err)
		errorResponse(c, http.StatusInternalServerError, "Failed to create documents")
		return
	}
	c.Status(http.StatusOK)
}

func searchEndpoint(c *gin.Context) {
	// get search term
	query := c.Query("search")
	if query == "" {
		errorResponse(c, http.StatusBadRequest, "Query not specified")
		return
	}
	skip := 0
	take := 10
	if i, err := strconv.Atoi(c.Query("skip")); err == nil {
		skip = i
	}
	if i, err := strconv.Atoi(c.Query("take")); err == nil {
		take = i
	}

	esQuery := elastic.
		NewMultiMatchQuery(query, "title", "description", "author").
		MinimumShouldMatch("2")
	result, err := elasticClient.Search().
		Index(elasticIndexName).
		Query(esQuery).
		From(skip).Size(take).
		Do(c.Request.Context())
	if err != nil {
		log.Println(err)
		errorResponse(c, http.StatusInternalServerError, "Something went wrong")
		return
	}
	res := SearchResponse{
		Time: fmt.Sprintf("%d", result.TookInMillis),
		Hits: fmt.Sprintf("%d", result.Hits.TotalHits),
	}
	// Map to response data
	docs := make([]DocumentResponse, 0)
	for _, hit := range result.Hits.Hits {
		var doc DocumentResponse
		json.Unmarshal(*hit.Source, &doc)
		docs = append(docs, doc)
	}
	res.Documents = docs
	c.JSON(http.StatusOK, res)
}

func errorResponse(c *gin.Context, code int, err string) {
	c.JSON(code, gin.H{
		"error": err,
	})
}

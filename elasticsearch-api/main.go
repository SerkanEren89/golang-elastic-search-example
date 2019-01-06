package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/olivere/elastic"
	"github.com/teris-io/shortid"
	"log"
	"net/http"
	"strconv"
	"time"
)

var (
	elasticClient *elastic.Client
)

const (
	elasticIndexName = "books"
	elasticTypeName  = "book"
)

type Book struct {
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

type BookRequest struct {
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

type BookResponse struct {
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
	Time  string         `json:"time"`
	Hits  string         `json:"hits"`
	Books []BookResponse `json:"books"`
}

func main() {
	var err error
	// Create Elastic client and wait for Elasticsearch to be ready
	// Start HTTP server
	r := gin.Default()
	r.POST("/documents", createBooksEndpoint)
	r.GET("/documents", searchEndpoint)
	if err = r.Run(":8080"); err != nil {
		log.Fatal(err)
	}

}

func createBooksEndpoint(context *gin.Context) {
	// Parse request
	var books []BookRequest
	if err := context.BindJSON(&books); err != nil {
		errorResponse(context, http.StatusBadRequest, "Malformed request body")
		return
	}
	// Insert documents in bulk
	bulk := elasticClient.
		Bulk().
		Index(elasticIndexName).
		Type(elasticTypeName)
	for _, b := range books {
		book := Book{
			ID:          shortid.MustGenerate(),
			ISBN:        b.ISBN,
			Title:       b.Title,
			SubTitle:    b.SubTitle,
			Author:      b.Author,
			Published:   b.Published,
			Publisher:   b.Publisher,
			Pages:       b.Pages,
			Description: b.Description,
			CreatedAt:   time.Now().UTC(),
		}
		bulk.Add(elastic.NewBulkIndexRequest().Id(book.ID).Doc(book))
	}
	if _, err := bulk.Do(context.Request.Context()); err != nil {
		log.Println(err)
		errorResponse(context, http.StatusInternalServerError, "Failed to create documents")
		return
	}
	context.Status(http.StatusOK)
}

func searchEndpoint(context *gin.Context) {
	query := context.Query("searchTerm")
	if query == "" {
		errorResponse(context, http.StatusBadRequest, "Query not specified")
		return
	}

	skip := 0
	take := 10
	if i, err := strconv.Atoi(context.Query("skip")); err == nil {
		skip = i
	}
	if i, err := strconv.Atoi(context.Query("take")); err == nil {
		take = i
	}
	// Perform search
	esQuery := elastic.NewMultiMatchQuery(query, "title", "description").
		Fuzziness("2").
		MinimumShouldMatch("2")
	result, err := elasticClient.Search().
		Index(elasticIndexName).
		Query(esQuery).
		From(skip).Size(take).
		Do(context.Request.Context())
	if err != nil {
		log.Println(err)
		errorResponse(context, http.StatusInternalServerError, "Something went wrong")
		return
	}
	res := SearchResponse{
		Time: fmt.Sprintf("%d", result.TookInMillis),
		Hits: fmt.Sprintf("%d", result.Hits.TotalHits),
	}
	// Transform search results before returning them
	books := make([]BookResponse, 0)
	for _, hit := range result.Hits.Hits {
		var book BookResponse
		json.Unmarshal(*hit.Source, &book)
		books = append(books, book)
	}
	res.Books = books
	context.JSON(http.StatusOK, res)
}

func errorResponse(c *gin.Context, code int, err string) {
	c.JSON(code, gin.H{
		"error": err,
	})
}

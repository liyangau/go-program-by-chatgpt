package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/olekukonko/tablewriter"
)

type Workspace struct {
	Name string `json:"name"`
	ID   string `json:"id"`
	// Add more fields as needed
}

type WorkspaceResponse struct {
	Data []Workspace `json:"data"`
}

type WorkspaceMetadata struct {
	WorkspaceName string
	Meta          Metadata
}

type Metadata struct {
	Counts struct {
		Plugins   int `json:"plugins"`
		Targets   int `json:"targets"`
		Services  int `json:"services"`
		Routes    int `json:"routes"`
		Upstreams int `json:"upstreams"`
	} `json:"counts"`
	// Add more fields as needed
}

func main() {
	// Parse command-line flags
	urlPtr := flag.String("kong-addr", "", "workspace URL (e.g. http://localhost:8001)")
	headersPtr := flag.String("headers", "", "headers for the request (e.g. 'x-admin-token:token_value')")
	flag.Parse()

	// Fallback to default URL if URL is empty
	if *urlPtr == "" {
		*urlPtr = os.Getenv("KONG_ADMIN_ADDR")
		if *urlPtr == "" {
			*urlPtr = "http://localhost:8001"
		}
	}

	// Set headers for the request
	headers := getHeaders(*headersPtr)

	// Send GET request to fetch workspaces
	workspacesURL := *urlPtr + "/workspaces"
	workspaces, err := getWorkspaces(workspacesURL)
	if err != nil {
		fmt.Println("Error getting workspaces:", err)
		return
	}

	// Initialize counts
	counts := make(map[string]int)

	// Iterate over workspaces and fetch metadata
	workspaceMetadataList := make([]WorkspaceMetadata, 0)

	for _, workspace := range workspaces {
		metaURL := *urlPtr + "/workspaces/" + workspace.Name + "/meta"
		meta, err := getMetadata(metaURL, headers)
		if err != nil {
			fmt.Printf("Error getting metadata for workspace %s: %v\n", workspace.Name, err)
			continue
		}

		// Update counts for each meta field
		updateCounts(meta, counts)

		// Store workspace metadata
		workspaceMetadata := WorkspaceMetadata{
			WorkspaceName: workspace.Name,
			Meta:          meta,
		}
		workspaceMetadataList = append(workspaceMetadataList, workspaceMetadata)
	}

	// Print individual workspace metadata
	fmt.Println("Individual Workspace Metadata:")
	printWorkspaceMetadataTable(workspaceMetadataList)

	// Print total counts
	fmt.Println("Total Entity Counts:")
	printCountsTable(counts, len(workspaces))
}

func getWorkspaces(url string) ([]Workspace, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response WorkspaceResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return response.Data, nil
}

func getMetadata(url string, headers http.Header) (Metadata, error) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Metadata{}, err
	}

	// Set
	req.Header = headers

	resp, err := client.Do(req)
	if err != nil {
		return Metadata{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Metadata{}, err
	}

	var meta Metadata
	err = json.Unmarshal(body, &meta)
	if err != nil {
		return Metadata{}, err
	}

	return meta, nil
}

func updateCounts(obj interface{}, counts map[string]int) {
	objValue := reflect.ValueOf(obj)
	objType := objValue.Type()

	for i := 0; i < objValue.NumField(); i++ {
		field := objType.Field(i)
		fieldValue := objValue.Field(i)

		switch fieldValue.Kind() {
		case reflect.Int:
			if fieldValue.Interface().(int) != 0 {
				counts[field.Name] += fieldValue.Interface().(int)
			}
		case reflect.Struct:
			updateCounts(fieldValue.Interface(), counts)
		}
	}
}

func printWorkspaceMetadataTable(workspaceMetadataList []WorkspaceMetadata) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Workspace", "Plugins", "Targets", "Services", "Routes", "Upstreams"})

	for _, workspaceMetadata := range workspaceMetadataList {
		meta := workspaceMetadata.Meta
		data := []string{
			workspaceMetadata.WorkspaceName,
			fmt.Sprint(meta.Counts.Plugins),
			fmt.Sprint(meta.Counts.Targets),
			fmt.Sprint(meta.Counts.Services),
			fmt.Sprint(meta.Counts.Routes),
			fmt.Sprint(meta.Counts.Upstreams),
		}
		table.Append(data)
	}

	table.Render()
}

func printCountsTable(counts map[string]int, workspaceCount int) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Meta Field", "Count"})

	for field, count := range counts {
		row := []string{
			field,
			fmt.Sprint(count),
		}
		table.Append(row)
	}

	// Add workspace count to the table
	table.Append([]string{"Workspaces", fmt.Sprint(workspaceCount)})

	table.Render()
}

func getHeaders(headersFlag string) http.Header {
	headers := make(http.Header)

	// Check if headersFlag is provided
	if headersFlag != "" {
		headerParts := strings.Split(headersFlag, ":")
		if len(headerParts) == 2 {
			headerKey := strings.TrimSpace(headerParts[0])
			headerValue := strings.TrimSpace(headerParts[1])
			headers.Set(headerKey, headerValue)
		}
	}

	// Check if X_ADMIN_TOKEN environment variable is set
	if token := os.Getenv("X_ADMIN_TOKEN"); token != "" {
		headers.Set("x-admin-token", token)
	}

	return headers
}

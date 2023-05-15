package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
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

type Metadata struct {
	Counts map[string]int `json:"counts"`
	// Add more fields as needed
}

type WorkspaceMetadata struct {
	WorkspaceName string
	Meta          Metadata
}

func main() {
	// Parse command-line flags
	urlPtr := flag.String("kong-addr", "", "workspace URL (e.g. http://localhost:8001)")
	headersPtr := flag.String("headers", "", "headers to include in the HTTP request")
	metaPtr := flag.String("meta", "counts", "metadata option: 'workspace', or 'all'")
	flag.Parse()

	// Fallback to default URL if URL is empty
	if *urlPtr == "" {
		*urlPtr = os.Getenv("KONG_ADMIN_ADDR")
		if *urlPtr == "" {
			*urlPtr = "http://localhost:8001"
		}
	}

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
		meta, err := getMetadata(metaURL, *headersPtr)
		if err != nil {
			fmt.Printf("Error getting metadata for workspace %s: %v\n", workspace.Name, err)
			continue
		}

		// Update counts for each meta field
		updateCounts(meta.Counts, counts)

		// Store workspace metadata
		workspaceMetadata := WorkspaceMetadata{
			WorkspaceName: workspace.Name,
			Meta:          meta,
		}
		workspaceMetadataList = append(workspaceMetadataList, workspaceMetadata)
	}

	// Print individual workspace metadata if specified
	if *metaPtr == "workspace" || *metaPtr == "all" {
		fmt.Println("Individual Workspace Metadata:")
		printWorkspaceMetadataTable(workspaceMetadataList)
	}

	// Print total counts if specified
	if *metaPtr == "counts" || *metaPtr == "all" {
		fmt.Println("Total Meta Field Counts:")
		printCountsTable(counts, len(workspaceMetadataList))
	}
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

func getMetadata(url string, headers string) (Metadata, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Metadata{}, err
	}

	// Add headers if provided
	if headers != "" {
		headerArr := strings.Split(headers, ":")
		if len(headerArr) == 2 {
			req.Header.Set(strings.TrimSpace(headerArr[0]), strings.TrimSpace(headerArr[1]))
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return Metadata{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Metadata{}, err
	}

	var metadata Metadata
	err = json.Unmarshal(body, &metadata)
	if err != nil {
		return Metadata{}, err
	}

	return metadata, nil
}

func updateCounts(metaCounts map[string]int, counts map[string]int) {
	for key, value := range metaCounts {
		if count, ok := counts[key]; ok {
			counts[key] = count + value
		} else {
			counts[key] = value
		}
	}
}

func printWorkspaceMetadataTable(metadataList []WorkspaceMetadata) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Workspace Name", "Plugins", "Targets", "Services", "Routes", "Upstreams"})

	for _, metadata := range metadataList {
		plugins := strconv.Itoa(metadata.Meta.Counts["plugins"])
		targets := strconv.Itoa(metadata.Meta.Counts["targets"])
		services := strconv.Itoa(metadata.Meta.Counts["services"])
		routes := strconv.Itoa(metadata.Meta.Counts["routes"])
		upstreams := strconv.Itoa(metadata.Meta.Counts["upstreams"])

		table.Append([]string{metadata.WorkspaceName, plugins, targets, services, routes, upstreams})
	}

	table.Render()
}

func printCountsTable(counts map[string]int, workspaceCount int) {
	// Create a slice of struct to hold the field and count information
	type MetaField struct {
		Field string
		Count int
	}

	metaFields := make([]MetaField, 0, len(counts))

	// Convert the map to a slice of MetaField structs
	for field, count := range counts {
		metaFields = append(metaFields, MetaField{Field: field, Count: count})
	}

	// Sort the metaFields slice based on the count in ascending order
	sort.Slice(metaFields, func(i, j int) bool {
		return metaFields[i].Count < metaFields[j].Count
	})

	// Print the sorted meta fields table
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Meta Field", "Count"})

	// Append the workspace count row to the table
	table.Append([]string{"Workspaces", strconv.Itoa(workspaceCount)})

	// Append the meta fields rows to the table
	for _, metaField := range metaFields {
		row := []string{
			metaField.Field,
			strconv.Itoa(metaField.Count),
		}
		table.Append(row)
	}

	table.Render()
}

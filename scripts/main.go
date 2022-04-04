package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"
)

var isDebug = false
var queryPerLoop = 3
var pageSize = 100
var outputPath = "../data/modules.csv"

func main() {
	var progressBar ProgressBar

	chEntry := make(chan Entry)
	chIsFinished := make(chan bool)

	urlsList := getUrlLists()

	progressBar.InitBar(0, len(urlsList))
	go progressBar.start()

	for i, url := range urlsList {
		if i%queryPerLoop == queryPerLoop-1 {
			time.Sleep(1 * time.Second)
		}
		progressBar.setCur(i + 1)
		go start(url, chEntry, chIsFinished)
	}

	var entries []Entry
	for i := 0; i < len(urlsList); {
		select {
		case e := <-chEntry:
			entries = append(entries, e)
		case <-chIsFinished:
			i++
		}
	}

	fmt.Println()
	writeToDisk(entries)
}

func fetchUrl(url string) (*http.Response, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		return resp, err
	}
	return resp, nil
}

func start(url string, chEntry chan Entry, chIsFinished chan bool) {
	defer func() {
		chIsFinished <- true
	}()

	resp, err := fetchUrl(url)
	if resp.StatusCode == 429 {
		log.Fatal("Hashicorp rate limited")
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var entries Entries
	err = json.Unmarshal([]byte(body), &entries)
	if err != nil {
		log.Fatal(err)
	}

	for _, e := range entries {
		chEntry <- e
	}
}

func getModuleCount() int {
	resp, _ := fetchUrl("https://registry.terraform.io/v2/modules?&page[size]=1")

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	tmp := struct {
		Meta struct {
			Pagination struct {
				TotalCount int `json:"total-count"`
			} `json:"pagination"`
		} `json:"meta"`
	}{}
	json.Unmarshal(body, &tmp)
	return tmp.Meta.Pagination.TotalCount
}

func getModuleTotalPage() int {
	if isDebug == true {
		return 1
	}
	return int(math.Ceil(float64(getModuleCount()) / float64(pageSize)))
}

func getUrlLists() []string {
	var res []string
	if isDebug {
		pageSize = 5
	}
	N := getModuleTotalPage()
	for i := 1; i < N+1; i++ {
		res = append(res, fmt.Sprintf("https://registry.terraform.io/v2/modules?page[size]=%d&page[number]=%d&include=latest-version", pageSize, i))
	}
	return res
}

func writeToDisk(entries Entries) {
	csvFile, err := os.Create(outputPath)
	if err != nil {
		log.Fatal(err)
	}
	defer csvFile.Close()
	writer := csv.NewWriter(csvFile)
	for _, entry := range entries {
		writer.Write(entry.Values())
	}
	writer.Flush()

	command := fmt.Sprintf("realpath %s", outputPath)
	outputFileAbsPath, err := exec.Command("bash", "-c", command).Output()
	if err != nil {
		log.Fatal(err)
	}

	command = fmt.Sprintf("sort -o %s %s", outputPath, outputPath)
	_, err = exec.Command("bash", "-c", command).Output()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("File written to %s", string(outputFileAbsPath))
}

type Entries []Entry
type Entry struct {
	Name         string
	Namespace    string
	ProviderName string
	Description  string
	Source       string
	ID           string
}

func (entry Entry) Values() []string {
	e := reflect.ValueOf(&entry).Elem()
	numField := e.NumField()

	res := make([]string, numField)
	for i := 0; i < numField; i++ {
		res[i] = fmt.Sprint(e.Field(i).Interface())
	}
	return res
}

func (e *Entries) UnmarshalJSON(data []byte) error {

	tmp := struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				FullName     string `json:"full-name"`
				Name         string `json:"name"`
				Namespace    string `json:"namespace"`
				ProviderName string `json:"provider-name"`
				Source       string `json:"source"`
			} `json:"attributes"`
			Relationships struct {
				LatestVersion struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"latest-version"`
			} `json:"relationships"`
		} `json:"data"`
		Included []struct {
			Id         string `json:"id"`
			Attributes struct {
				Description string `json:"description"`
			} `json:"attributes"`
		} `json:"included"`
	}{}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	moduleVersions := make(map[string]string)
	for _, moduleVersion := range tmp.Included {
		moduleVersions[moduleVersion.Id] = moduleVersion.Attributes.Description
	}

	for _, d := range tmp.Data {
		*e = append(*e, Entry{
			Name:         d.Attributes.Name,
			Namespace:    d.Attributes.Namespace,
			ProviderName: d.Attributes.ProviderName,
			Description:  moduleVersions[d.Relationships.LatestVersion.Data.ID],
			Source:       d.Attributes.Source,
			ID:           d.ID,
		})
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////
// Progress Bar
////////////////////////////////////////////////////////////////////////////////
type ProgressBar struct {
	cur         int    // current progress
	percent     int    // current progress
	total       int    // total value for progress
	graph       string // the fill value for progress bar
	refreshRate time.Duration
}

func (bar *ProgressBar) setCur(cur int) {
	bar.cur = cur
	bar.percent = bar.getPercent()
}

func (bar *ProgressBar) InitBar(cur int, total int) {
	bar.cur = cur
	bar.total = total
	bar.percent = bar.getPercent()
	bar.graph = "#"
	bar.refreshRate = 200 * time.Millisecond
}

func (bar *ProgressBar) getPercent() int {
	return int((float32(bar.cur) / float32(bar.total)) * 100)
}

func (bar *ProgressBar) getCurrentProgressBar() string {
	return strings.Repeat("#", bar.percent/2)
}

func (bar *ProgressBar) print(elapsed time.Duration) {
	fmt.Printf("\r[%-50s] %3d%%  %8d/%d [%ds]", bar.getCurrentProgressBar(), bar.percent, bar.cur, bar.total, int(elapsed.Seconds()))
}

func (bar *ProgressBar) start() {
	start := time.Now()
	for bar.cur < bar.total {
		time.Sleep(bar.refreshRate)
		bar.print(time.Since(start))
	}
	bar.print(time.Since(start))
}

package watttime

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

type LoginInput struct {
	Username string
	Password string
}

type LoginOutput struct {
	Token string
}

type GridRegionsInput struct {
	Token string
	All   bool
}

type GridRegionsOutput struct {
	GridRegions []GridRegion
}

type IndexInput struct {
	Token string
	Ba    string
}

type IndexOutput struct {
	Freq      string    `json:"freq"`
	Ba        string    `json:"ba"`
	Percent   int       `json:"percent"`
	Moer      float64   `json:"moer"`
	PointTime time.Time `json:"point_time"`
}

type IndexResponse struct {
	Freq      string `json:"freq"`
	Ba        string `json:"ba"`
	Percent   string `json:"percent"`
	Moer      string `json:"moer"`
	PointTime string `json:"point_time"`
}

type GridRegion struct {
	Ba       string `json:"ba"`
	Name     string `json:"name"`
	Access   string `json:"access"`
	Datatype string `json:"datatype"`
}

type Result []interface{}

type WattLoginResponse struct {
	Token string `json:"token"`
}

func Login(input *LoginInput) (*LoginOutput, error) {
	output := &LoginOutput{}

	h := http.Client{}
	req, _ := http.NewRequest("GET", "https://api2.watttime.org/v2/login", nil)
	req.SetBasicAuth(input.Username, input.Password)
	r, err := h.Do(req)
	if err != nil {
		return nil, err
	} else if r.Status != "200 OK" {
		return nil, errors.New("Status: " + r.Status)
	}

	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	wattRes := &WattLoginResponse{}
	err = json.Unmarshal(body, wattRes)
	if err != nil {
		return nil, err
	}

	output.Token = wattRes.Token

	return output, nil
}

func GridRegions(input *GridRegionsInput) (*GridRegionsOutput, error) {
	output := &GridRegionsOutput{}
	url := "https://api2.watttime.org/v2/ba-access"
	if input.All {
		url += "?all=true"
	}

	h := http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+input.Token)
	r, err := h.Do(req)
	if err != nil {
		return nil, err
	} else if r.Status != "200 OK" {
		return nil, errors.New("Status: " + r.Status)
	}

	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	var wattRes Result
	err = json.Unmarshal(body, &wattRes)
	if err != nil {
		return nil, err
	}

	typedResult := make([]GridRegion, len(wattRes))
	for index, item := range wattRes {
		var gridRegion GridRegion
		b, _ := json.Marshal(item)
		json.Unmarshal(b, &gridRegion)
		typedResult[index] = gridRegion
	}

	output.GridRegions = typedResult

	return output, nil
}

func GetIndex(input *IndexInput) (*IndexOutput, error) {

	output := &IndexOutput{}
	url := "https://api2.watttime.org/index?ba=" + input.Ba

	h := http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+input.Token)
	r, err := h.Do(req)
	if err != nil {
		return nil, err
	} else if r.Status != "200 OK" {
		return nil, errors.New("Status: " + r.Status)
	}

	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	indexRes := &IndexResponse{}
	err = json.Unmarshal(body, indexRes)
	if err != nil {
		return nil, err
	}

	output.Freq = indexRes.Freq
	output.Ba = indexRes.Ba
	output.Percent, _ = strconv.Atoi(indexRes.Percent)
	output.Moer, _ = strconv.ParseFloat(indexRes.Moer, 64)
	output.PointTime, _ = time.Parse(time.RFC3339, indexRes.PointTime)

	return output, nil

}

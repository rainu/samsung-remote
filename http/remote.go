package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type SamsungRemote interface {
	//Provides information about the Samsung TV. Usually this is a json-string.
	GetInformation() (string, error)

	//Provides information about the Samsung TV. Usually this is a json-string.
	//Cancels the action if the passed context is closed.
	GetInformationWithContext(ctx context.Context) (string, error)

	//Starts a app with the given name. You can provide some data to these app.
	//For example, you can start the YouTube app and play a video directly (set data to "v=HvncJgJbqOc")
	StartApp(name string, data []byte) error

	//Starts a app with the given name. You can provide some data to these app.
	//For example, you can start the YouTube app and play a video directly (set data to "v=HvncJgJbqOc")
	//Cancels the action if the passed context is closed.
	StartAppWithContext(name string, data []byte, ctx context.Context) error

	//Starts a app with the given app-id.
	StartAppById(appId string) error

	//Starts a app with the given app-id.
	//Cancels the action if the passed context is closed.
	StartAppByIdWithContext(appId string, ctx context.Context) error

	//Get the status by the given app id.
	GetAppStatus(appId string) (ApplicationStatus, error)

	//Get the status by the given app id.
	//Cancels the action if the passed context is closed.
	GetAppStatusWithContext(appId string, ctx context.Context) (ApplicationStatus, error)

	//Close the app by the given id.
	CloseApp(appId string) error

	//Close the app by the given id.
	//Cancels the action if the passed context is closed.
	CloseAppWithContext(appId string, ctx context.Context) error

	Close() error
}

type ApplicationStatus struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	IsRunning bool   `json:"running"`
	Version   string `json:"version"`
	IsVisible bool   `json:"visible"`
}

type SamsungRemoteConfig struct {
	//The base url to be used for the http connection. Typically this is one of these:
	//
	// http://tv_ip:8001
	//
	// https://tv_ip:8002
	BaseUrl string

	//Optional. The http client to use.
	HttpClient *http.Client
}

type appLauncher struct {
	baseUrl string
	client  *http.Client
}

//Create a new instance of http-based SamsungRemote.
func NewRemote(conf SamsungRemoteConfig) SamsungRemote {
	if conf.HttpClient == nil {
		conf.HttpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			Timeout: 30 * time.Second,
		}
	}

	return &appLauncher{
		baseUrl: conf.BaseUrl,
		client:  conf.HttpClient,
	}
}

func (a *appLauncher) GetInformation() (string, error) {
	return a.GetInformationWithContext(context.Background())
}

func (a *appLauncher) GetInformationWithContext(ctx context.Context) (string, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v2/", a.baseUrl), nil)
	if err != nil {
		return "", fmt.Errorf("unable to build request: %w", err)
	}
	req = req.WithContext(ctx)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error while sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("error while reading request: %w", err)
		}
		return string(bodyBytes), nil
	}
	return "", fmt.Errorf("received bad status code: %d", resp.StatusCode)
}

func (a *appLauncher) StartApp(appId string, appData []byte) error {
	return a.StartAppWithContext(appId, appData, context.Background())
}

func (a *appLauncher) StartAppWithContext(appId string, appData []byte, ctx context.Context) error {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/ws/apps/%s", a.baseUrl, appId), bytes.NewBuffer(appData))
	if err != nil {
		return fmt.Errorf("unable to build request: %w", err)
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(appData)))

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("error while sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return nil
	}
	return fmt.Errorf("received bad status code: %d", resp.StatusCode)
}

func (a *appLauncher) StartAppById(appId string) error {
	return a.StartAppByIdWithContext(appId, context.Background())
}

func (a *appLauncher) StartAppByIdWithContext(appId string, ctx context.Context) error {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v2/applications/%s", a.baseUrl, appId), nil)
	if err != nil {
		return fmt.Errorf("unable to build request: %w", err)
	}
	req = req.WithContext(ctx)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("error while sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return nil
	}
	return fmt.Errorf("received bad status code: %d", resp.StatusCode)
}

func (a *appLauncher) CloseApp(appId string) error {
	return a.CloseAppWithContext(appId, context.Background())
}

func (a *appLauncher) CloseAppWithContext(appId string, ctx context.Context) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v2/applications/%s", a.baseUrl, appId), nil)
	if err != nil {
		return fmt.Errorf("unable to build request: %w", err)
	}
	req = req.WithContext(ctx)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("error while sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return nil
	}
	return fmt.Errorf("received bad status code: %d", resp.StatusCode)
}

func (a *appLauncher) GetAppStatus(appId string) (ApplicationStatus, error) {
	return a.GetAppStatusWithContext(appId, context.Background())
}

func (a *appLauncher) GetAppStatusWithContext(appId string, ctx context.Context) (ApplicationStatus, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v2/applications/%s", a.baseUrl, appId), nil)
	if err != nil {
		return ApplicationStatus{}, fmt.Errorf("unable to build request: %w", err)
	}
	req = req.WithContext(ctx)

	resp, err := a.client.Do(req)
	if err != nil {
		return ApplicationStatus{}, fmt.Errorf("error while sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		var result ApplicationStatus
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return ApplicationStatus{}, fmt.Errorf("could not decode response: %w", err)
		}
		return result, nil
	}
	return ApplicationStatus{}, fmt.Errorf("received bad status code: %d", resp.StatusCode)
}

func (a *appLauncher) Close() error {
	//nothing at the moment
	return nil
}

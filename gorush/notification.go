package gorush

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	_"os"
	"path/filepath"
	"sync"
	"time"

	"strings"

	"github.com/google/go-gcm"
	apns "github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"

	"github.com/NaySoftware/go-fcm"
)

// D provide string array
type D map[string]interface{}

// Push response
type PushResponse struct {
	Status       string   `json:"status,omitempty"`
	CanonicalId  string   `json:"canonical_id,omitempty"`
	Error        string   `json:"error,omitempty"`
}

const (
	// ApnsPriorityLow will tell APNs to send the push message at a time that takes
	// into account power considerations for the device. Notifications with this
	// priority might be grouped and delivered in bursts. They are throttled, and
	// in some cases are not delivered.
	ApnsPriorityLow = 5

	// ApnsPriorityHigh will tell APNs to send the push message immediately.
	// Notifications with this priority must trigger an alert, sound, or badge on
	// the target device. It is an error to use this priority for a push
	// notification that contains only the content-available key.
	ApnsPriorityHigh = 10
)

// Alert is APNs payload
type Alert struct {
	Action       string   `json:"action,omitempty"`
	ActionLocKey string   `json:"action-loc-key,omitempty"`
	Body         string   `json:"body,omitempty"`
	LaunchImage  string   `json:"launch-image,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
	LocKey       string   `json:"loc-key,omitempty"`
	Title        string   `json:"title,omitempty"`
	Subtitle     string   `json:"subtitle,omitempty"`
	TitleLocArgs []string `json:"title-loc-args,omitempty"`
	TitleLocKey  string   `json:"title-loc-key,omitempty"`
}

// RequestPush support multiple notification request.
type RequestPush struct {
	Notifications []PushNotification `json:"notifications" binding:"required"`
}

// PushNotification is single notification request
type PushNotification struct {
	// Common
	Tokens           []string `json:"tokens" binding:"required"`
	Platform         int      `json:"platform" binding:"required"`
	Message          string   `json:"message,omitempty"`
	Title            string   `json:"title,omitempty"`
	Priority         string   `json:"priority,omitempty"`
	ContentAvailable bool     `json:"content_available,omitempty"`
	Sound            string   `json:"sound,omitempty"`
	Data             D        `json:"data,omitempty"`
	AppID            string   `json:"data,omitempty"`
	Retry            int      `json:"retry,omitempty"`
	wg               *sync.WaitGroup

	// Android
	APIKey                string           `json:"api_key,omitempty"`
	To                    string           `json:"to,omitempty"`
	CollapseKey           string           `json:"collapse_key,omitempty"`
	DelayWhileIdle        bool             `json:"delay_while_idle,omitempty"`
	TimeToLive            *uint            `json:"time_to_live,omitempty"`
	RestrictedPackageName string           `json:"restricted_package_name,omitempty"`
	DryRun                bool             `json:"dry_run,omitempty"`
	Notification          gcm.Notification `json:"notification,omitempty"`
	AndroidData           D                `json:"android_data,omitempty"`

	// iOS
	Expiration     int64    `json:"expiration,omitempty"`
	ApnsID         string   `json:"apns_id,omitempty"`
	Topic          string   `json:"topic,omitempty"`
	Badge          *int     `json:"badge,omitempty"`
	Category       string   `json:"category,omitempty"`
	URLArgs        []string `json:"url-args,omitempty"`
	Alert          Alert    `json:"alert,omitempty"`
	MutableContent bool     `json:"mutable-content,omitempty"`
	IosData    	   D        `json:"ios_data,omitempty"`
}

// Done decrements the WaitGroup counter.
func (p *PushNotification) Done() {
	if p.wg != nil {
		p.wg.Done()
	}
}

// ApnsClients is collection of apns client connections
type ApnsClients struct {
	lock    sync.RWMutex
	clients map[string]*apns.Client
}

var apnsClients = &ApnsClients{}

// FcmClients is collection of FCM client connections
type FcmClients struct {
	lock    sync.RWMutex
	clients map[string]*fcm.FcmClient
}

var fcmClients = &FcmClients{}

// CheckMessage for check request message
func CheckMessage(req PushNotification) error {
	var msg string

	if len(req.Tokens) == 0 {
		msg = "the message must specify at least one registration ID"
		LogAccess.Debug(msg)
		return errors.New(msg)
	}

	if len(req.Tokens) == PlatFormIos && len(req.Tokens[0]) == 0 {
		msg = "the token must not be empty"
		LogAccess.Debug(msg)
		return errors.New(msg)
	}

	if req.Platform == PlatFormAndroid && len(req.Tokens) > 1000 {
		msg = "the message may specify at most 1000 registration IDs"
		LogAccess.Debug(msg)
		return errors.New(msg)
	}

	// ref: https://developers.google.com/cloud-messaging/http-server-ref
	if req.Platform == PlatFormAndroid && req.TimeToLive != nil && (*req.TimeToLive < uint(0) || uint(2419200) < *req.TimeToLive) {
		msg = "the message's TimeToLive field must be an integer " +
			"between 0 and 2419200 (4 weeks)"
		LogAccess.Debug(msg)
		return errors.New(msg)
	}

	return nil
}

// SetProxy only working for GCM server.
func SetProxy(proxy string) error {

	proxyURL, err := url.ParseRequestURI(proxy)

	if err != nil {
		return err
	}

	http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	LogAccess.Debug("Set http proxy as " + proxy)

	return nil
}

// CheckPushConf provide check your yml config.
// To be reimplemented later
func CheckPushConf() error {
	/*if !PushConf.Ios.Enabled && !PushConf.Android.Enabled {
		return errors.New("Please enable iOS or Android config in yml config")
	}

	if PushConf.Ios.Enabled {
		if PushConf.Ios.KeyPath == "" {
			return errors.New("Missing iOS certificate path")
		}

		// check certificate file exist
		if _, err := os.Stat(PushConf.Ios.KeyPath); os.IsNotExist(err) {
			return errors.New("certificate file does not exist")
		}
	}

	if PushConf.Android.Enabled {
		if PushConf.Android.APIKey == "" {
			return errors.New("Missing Android API Key")
		}
	}*/

	return nil
}

// initAPNSClient initializes an APNs Client for the given AppID.
func initAPNSClient(AppID string) (*apns.Client, error) {
	var err error
	var apnsClient *apns.Client

	if PushConf.Apps[AppID].Ios.Enabled {

		ext := filepath.Ext(PushConf.Apps[AppID].Ios.KeyPath)

		// Append the certificates dir for the path
		IosKeyPath := PushConf.Apps[AppID].Ios.KeyPath
		CertCommonDir := PushConf.Core.CertDir
		if (len(strings.TrimSpace(CertCommonDir)) != 0) {

			IosKeyPath = CertCommonDir + IosKeyPath
		}

		switch ext {
		case ".p12":
			CertificatePemIos, err = certificate.FromP12File(IosKeyPath, PushConf.Apps[AppID].Ios.Password)
		case ".pem":
			CertificatePemIos, err = certificate.FromPemFile(IosKeyPath, PushConf.Apps[AppID].Ios.Password)
		default:
			err = errors.New("wrong certificate key extension")
		}

		if err != nil {
			LogError.Error("Cert Error:", err.Error())

			return nil, err
		}

		if PushConf.Apps[AppID].Ios.Production {
			apnsClient = apns.NewClient(CertificatePemIos).Production()
		} else {
			apnsClient = apns.NewClient(CertificatePemIos).Development()
		}
	}

	return apnsClient, nil
}

// initFCMClient initializes an FCM Client for the given AppID.
func initFCMClient(AppID string) (*fcm.FcmClient, error) {
	var err error
	var fcmClient *fcm.FcmClient

	if PushConf.Apps[AppID].AndroidFcm.Enabled {

		apiKey := PushConf.Apps[AppID].AndroidFcm.APIKey

		fcmClient = fcm.NewFcmClient(apiKey)

		return fcmClient, nil
	}

	err = errors.New("FCM not enabled")

	return nil, err
}

// GetFCMClient returns an existing FCM client connection if available else
// creates a new connection and returns
func GetFcmClient(AppID string) (*fcm.FcmClient, error) {
    client, err := initFCMClient(AppID)
	return client, err
}

// GetAPNSClient returns an existing APNs client connection if available else
// creates a new connection and returns
//
// For faster concurrency with locks, double checks have been used
// (https://www.misfra.me/optimizing-concurrent-map-access-in-go/)
func GetAPNSClient(AppID string) (*apns.Client, error) {
	var client *apns.Client
	var present bool
	var err error

	if len(apnsClients.clients) == 0 {
		apnsClients.clients = make(map[string]*apns.Client, 0)
	}

	apnsClients.lock.RLock()
	if client, present = apnsClients.clients[AppID]; !present {
		// The connection wasn't found, so we'll create it.
		apnsClients.lock.RUnlock()
		apnsClients.lock.Lock()
		if client, present = apnsClients.clients[AppID]; !present {
			client, err = initAPNSClient(AppID)

			apnsClients.clients[AppID] = client
		}
		apnsClients.lock.Unlock()
	} else {
		apnsClients.lock.RUnlock()
	}

	return client, err
}

// InitWorkers for initialize all workers.
func InitWorkers(workerNum int64, queueNum int64) {
	LogAccess.Debug("worker number is ", workerNum, ", queue number is ", queueNum)
	QueueNotification = make(chan PushNotification, queueNum)
	for i := int64(0); i < workerNum; i++ {
		go startWorker()
	}
}

func startWorker() {
	for {
		notification := <-QueueNotification
		switch notification.Platform {
		case PlatFormIos:
			PushToIOS(notification)
		case PlatFormAndroid:
			PushToAndroid(notification)
		}
	}
}

// queueNotification add notification to queue list.
func queueNotification(req RequestPush) int {
	var count int
	wg := sync.WaitGroup{}
	for _, notification := range req.Notifications {

		// send notification to `normal` app, if app not specified
		if notification.AppID == "" {
			notification.AppID = AppNameDefault
		}

		// skip notification if unkown app specified
		if _, exists := PushConf.Apps[notification.AppID]; !exists {
			LogError.Error("Unknown app: " + notification.AppID)
			continue
		}

		switch notification.Platform {
		case PlatFormIos:
			if !PushConf.Apps[notification.AppID].Ios.Enabled {
				continue
			}
		case PlatFormAndroid:
			if !PushConf.Apps[notification.AppID].Android.Enabled {
				continue
			}
		}
		wg.Add(1)
		notification.wg = &wg
		QueueNotification <- notification
		count += len(notification.Tokens)
	}

	if PushConf.Core.Sync {
		wg.Wait()
	}

	StatStorage.AddTotalCount(int64(count))

	return count
}

func iosAlertDictionary(payload *payload.Payload, req PushNotification) *payload.Payload {
	// Alert dictionary

	if len(req.Title) > 0 {
		payload.AlertTitle(req.Title)
	}

	if len(req.Alert.Title) > 0 {
		payload.AlertTitle(req.Alert.Title)
	}

	// Apple Watch & Safari display this string as part of the notification interface.
	if len(req.Alert.Subtitle) > 0 {
		payload.AlertSubtitle(req.Alert.Subtitle)
	}

	if len(req.Alert.TitleLocKey) > 0 {
		payload.AlertTitleLocKey(req.Alert.TitleLocKey)
	}

	if len(req.Alert.LocArgs) > 0 {
		payload.AlertLocArgs(req.Alert.LocArgs)
	}

	if len(req.Alert.TitleLocArgs) > 0 {
		payload.AlertTitleLocArgs(req.Alert.TitleLocArgs)
	}

	if len(req.Alert.Body) > 0 {
		payload.AlertBody(req.Alert.Body)
	}

	if len(req.Alert.LaunchImage) > 0 {
		payload.AlertLaunchImage(req.Alert.LaunchImage)
	}

	if len(req.Alert.LocKey) > 0 {
		payload.AlertLocKey(req.Alert.LocKey)
	}

	if len(req.Alert.Action) > 0 {
		payload.AlertAction(req.Alert.Action)
	}

	if len(req.Alert.ActionLocKey) > 0 {
		payload.AlertActionLocKey(req.Alert.ActionLocKey)
	}

	// General

	if len(req.Category) > 0 {
		payload.Category(req.Category)
	}

	return payload
}

// GetIOSNotification use for define iOS notification.
// The iOS Notification Payload
// ref: https://developer.apple.com/library/content/documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/PayloadKeyReference.html#//apple_ref/doc/uid/TP40008194-CH17-SW1
func GetIOSNotification(req PushNotification) *apns.Notification {
	notification := &apns.Notification{
		ApnsID: req.ApnsID,
		Topic:  req.Topic,
	}

	if req.Expiration > 0 {
		notification.Expiration = time.Unix(req.Expiration, 0)
	}

	if len(req.Priority) > 0 && req.Priority == "normal" {
		notification.Priority = apns.PriorityLow
	}

	payload := payload.NewPayload()

	// add alert object if message length > 0
	if len(req.Message) > 0 {
		payload.Alert(req.Message)
	}

	// zero value for clear the badge on the app icon.
	if req.Badge != nil && *req.Badge >= 0 {
		payload.Badge(*req.Badge)
	}

	if req.MutableContent {
		payload.MutableContent()
	}

	if len(req.Sound) > 0 {
		payload.Sound(req.Sound)
	}

	if req.ContentAvailable {
		payload.ContentAvailable()
	}

	if len(req.URLArgs) > 0 {
		payload.URLArgs(req.URLArgs)
	}

	// Get Common data fields
	for k, v := range req.Data {
		payload.Custom(k, v)
	}

	// Get ios specific data fields
	for k, v := range req.IosData {
		payload.Custom(k, v)
	}

	payload = iosAlertDictionary(payload, req)

	notification.Payload = payload

	return notification
}

// PushToIOS provide send notification to APNs server.
func PushToIOS(req PushNotification) map[string]*PushResponse {
	LogAccess.Debug("Start push notification for iOS")
	defer req.Done()
	var retryCount = 0
	var maxRetry = PushConf.Apps[req.AppID].Ios.MaxRetry

	pushResponse := make(map[string]*PushResponse, 0)

	if req.Retry > 0 && req.Retry < maxRetry {
		maxRetry = req.Retry
	}

Retry:
	var isError = false
	var newTokens []string

	notification := GetIOSNotification(req)

	// get apns client
	apnsClient, err := GetAPNSClient(req.AppID)
	if err != nil {
		LogPush(FailedPush, "", req, err)
		isError = true
		return pushResponse
	}

	for _, token := range req.Tokens {
		notification.DeviceToken = token

		// send ios notification
		res, err := apnsClient.Push(notification)

		pushResponse[token] = &PushResponse{
			Status:                "success",
			CanonicalId:           "",
			Error:                 "",
		}

		if err != nil {
			// apns server error

			pushResponse[token].Status = "apn_error"
			pushResponse[token].Error  = err.Error()

			LogPush(FailedPush, token, req, err)
			StatStorage.AddIosError(1)
			newTokens = append(newTokens, token)
			isError = true
			continue
		}

		if res.StatusCode != 200 {
			// error message:
			// ref: https://github.com/sideshow/apns2/blob/master/response.go#L14-L65

			pushResponse[token].Status = "failed"
			pushResponse[token].Error  = res.Reason

			LogPush(FailedPush, token, req, errors.New(res.Reason))
			StatStorage.AddIosError(1)
			newTokens = append(newTokens, token)
			isError = true
			continue
		}

		if res.Sent() {

			LogPush(SucceededPush, token, req, nil)
			StatStorage.AddIosSuccess(1)
		}
	}

	if isError == true && retryCount < maxRetry {
		retryCount++

		// resend fail token
		req.Tokens = newTokens
		goto Retry
	}

	return pushResponse
}

// PushToAndroidFcm provide send notification through FCM.
func PushToAndroidFcm(req PushNotification) map[string]*PushResponse {
	LogAccess.Debug("Start push notification for FCM")
	defer req.Done()
	var retryCount = 0
	var maxRetry = PushConf.Apps[req.AppID].AndroidFcm.MaxRetry

	pushResponse := make(map[string]*PushResponse, 0)

	if req.Retry > 0 && req.Retry < maxRetry {
		maxRetry = req.Retry
	}

Retry:
	var isError = false
	var newTokens []string

	notification, data := GetFcmNotification(req)

	// get fcm client
	fcmClient, err := GetFcmClient(req.AppID)
	if err != nil {
		LogPush(FailedPush, "", req, err)
		isError = true
		return pushResponse
	}

	for _, token := range req.Tokens {

		// new fcm msg
		fcmClient.NewFcmMsgTo(token, data)
		fcmClient.SetNotificationPayload(notification)

		// Send fcm msg
		res, err := fcmClient.Send()

		pushResponse[token] = &PushResponse{
			Status:                "success",
			CanonicalId:           "",
			Error:                 "",
		}

		if err != nil {
			// fcm error
			pushResponse[token].Status = "failed"
			pushResponse[token].Error  = err.Error()

			LogPush(FailedPush, token, req, err)
			newTokens = append(newTokens, token)
			isError = true
			continue
		}

		if res.Ok {
			LogPush(SucceededPush, token, req, nil)
		}
	}

	if isError == true && retryCount < maxRetry {
		retryCount++

		// resend fail token
		req.Tokens = newTokens
		goto Retry
	}

	return pushResponse
}

// GetFcmNotification use for define FCM notification.
// HTTP Connection Server Reference for FCM
// https://github.com/NaySoftware/go-fcm/blob/master/fcm.go#L75
// https://firebase.google.com/docs/cloud-messaging/http-server-ref
func GetFcmNotification(req PushNotification) (*fcm.NotificationPayload, interface{}) {
	notification := new(fcm.NotificationPayload)
	data := make(map[string]interface{})

	// Add another field
	if (len(req.Data) > 0 || len(req.AndroidData) > 0) {

		// Get Common data fields
		for k, v := range req.Data {
			data[k] = v
		}

		// Get platform specific data fields
		for k, v := range req.AndroidData {
			data[k] = v
		}
	}

	// Set request message if body is empty
	if len(req.Message) > 0 {
		notification.Body = req.Message
		data["Body"] = req.Message
	}

	if len(req.Title) > 0 {
		notification.Title = req.Title
		data["Title"] = req.Title
	}

	if len(req.Sound) > 0 {
		notification.Sound = req.Sound
		data["Sound"] = req.Sound
	}

	return notification, data
}

// GetAndroidNotification use for define Android notification.
// HTTP Connection Server Reference for Android
// https://developers.google.com/cloud-messaging/http-server-ref
func GetAndroidNotification(req PushNotification) gcm.HttpMessage {
	notification := gcm.HttpMessage{
		To:                    req.To,
		CollapseKey:           req.CollapseKey,
		ContentAvailable:      req.ContentAvailable,
		DelayWhileIdle:        req.DelayWhileIdle,
		TimeToLive:            req.TimeToLive,
		RestrictedPackageName: req.RestrictedPackageName,
		DryRun:                req.DryRun,
	}

	notification.RegistrationIds = req.Tokens

	if len(req.Priority) > 0 && req.Priority == "high" {
		notification.Priority = "high"
	}

	// Add another field
	if (len(req.Data) > 0 || len(req.AndroidData) > 0) {
		notification.Data = make(map[string]interface{})

		// Get Common data fields
		for k, v := range req.Data {
			notification.Data[k] = v
		}

		// Get platform specific data fields
		for k, v := range req.AndroidData {
			notification.Data[k] = v
		}
	}

	notification.Notification = &req.Notification

	// Set request message if body is empty
	if len(req.Message) > 0 {
		notification.Notification.Body = req.Message
	}

	if len(req.Title) > 0 {
		notification.Notification.Title = req.Title
	}

	if len(req.Sound) > 0 {
		notification.Notification.Sound = req.Sound
	}

	return notification
}

// PushToAndroid provide send notification to Android server.
func PushToAndroid(req PushNotification) map[string]*PushResponse {
	LogAccess.Debug("Start push notification for Android")

	defer req.Done()

	var retryCount = 0
	var maxRetry = PushConf.Apps[req.AppID].Android.MaxRetry

	if req.Retry > 0 && req.Retry < maxRetry {
		maxRetry = req.Retry
	}

	pushResponse := make(map[string]*PushResponse, 0)

	// Set api key if none provided in req
	var apiKey = req.APIKey
	if apiKey == "" {
		apiKey = PushConf.Apps[req.AppID].Android.APIKey
	}

	// check message
	err := CheckMessage(req)

	if err != nil {
		LogError.Error("request error: " + err.Error())
		return pushResponse
	}

Retry:
	var isError = false
	notification := GetAndroidNotification(req)

	res, err := gcm.SendHttp(apiKey, notification)

	if err != nil {
		// GCM server error
		LogError.Error("GCM server error: " + err.Error())
		return pushResponse
	}

	LogAccess.Debug(fmt.Sprintf("Android Success count: %d, Failure count: %d", res.Success, res.Failure))
	StatStorage.AddAndroidSuccess(int64(res.Success))
	StatStorage.AddAndroidError(int64(res.Failure))

	var newTokens []string
	for k, result := range res.Results {

		pushResponse[req.Tokens[k]] = &PushResponse{
			Status:                "success",
			CanonicalId:           "",
			Error:                 "",
		}

		if result.RegistrationId != "" {

			pushResponse[req.Tokens[k]].CanonicalId = result.RegistrationId
		}

		if result.Error != "" {
			isError = true
			newTokens = append(newTokens, req.Tokens[k])

			pushResponse[req.Tokens[k]].Status = "failed"
			pushResponse[req.Tokens[k]].Error = result.Error

			LogPush(FailedPush, req.Tokens[k], req, errors.New(result.Error))
			continue
		}

		LogPush(SucceededPush, req.Tokens[k], req, nil)
	}

	if isError == true && retryCount < maxRetry {
		retryCount++

		// resend fail token
		req.Tokens = newTokens
		goto Retry
	}

	return pushResponse
}

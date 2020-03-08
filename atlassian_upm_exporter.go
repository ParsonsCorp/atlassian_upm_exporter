package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	baseURL         string
	bearer          string
	disableCol      = true
	exporterName    = "Atlassian UPM Exporter"
	metricNamespace = "atlassian_upm"

	address          = flag.String("svc.ip-address", "0.0.0.0", "assign an IP address for the service to listen on")
	checkUpdates     = flag.Bool("check-updates", false, "check for updates available for each plugin. (1 connection per plugin)")
	debug            = flag.Bool("debug", false, "enable the service debug output")
	dropJiraSoftware = flag.Bool("drop-jira-software-plugins", false, "remove plugins vendored by Atlassian when monitoring Jira")
	dropDisabled     = flag.Bool("drop-disabled", false, "remove plugins if they are disabled")
	enableColLogs    = flag.Bool("enable-color-logs", false, "when developing in debug mode, prettier to set this for visual colors")
	fqdn             = flag.String("app.fqdn", "", "REQUIRED: provide the application fqdn to be monitored (ie. bitbucket.domain.com)")
	help             = flag.Bool("help", false, "help will display this helpful dialog output")
	port             = flag.String("svc.port", "9996", "can pass in the port to listen on.")
	protocol         = flag.String("app.protocol", "https", "set the protocol used to interact with the application")
	token            = flag.String("app.token", "", "REQUIRED: provide a Basic access token to connect with")
	userInstalled    = flag.Bool("user-installed", false, "if you would like 'user-installed' plugins only")

	usageMessage = "The Atlassian UPM (Universal Plugin Manager) Exporter is used to get the list\n" +
		"of plugins installed on the monitored system. Currently Bitbucket, Confluence\n" +
		"and Jira use the UPM to manage their plugins. These systems can be monitored\n" +
		"with this exporter, create one per application. The account that this container\n" +
		"will use to reach out and scrape will need to be a product Administrator to\n" +
		"that monitored Atlassian application.\n" +
		"\nMetrics Example:\n" +
		"atlassian_upm_collect_duration_seconds{url=''} 0\n" +
		"atlassian_upm_plugin{enabled='',installedVersion='',name='',url='',userInstalled=''} 0\n" +
		"atlassian_upm_plugin_version_available{availableVersion='',enabled='',installedVersion='',name='',url='',userInstalled=''} 0\n" +
		"atlassian_upm_rest_url_up{url=''} 0\n" +
		"\nReference:\n" +
		"https://confluence.atlassian.com/upm/universal-plugin-manager-documentation-273875696.html\n" +
		"\nUsage of atlassian_upm_exporter [Arguments]\n" +
		"\nArguments:\n"
)

// usage is a function used to display this binaries usage description and then exit the program.
var usage = func() {
	fmt.Println(usageMessage)
	flag.PrintDefaults()
	os.Exit(0)
}

// atlassianUPMCollector is the structure of our prometheus collector containing it descriptors.
type atlassianUPMCollector struct {
	atlassianUPMTimeMetric     *prometheus.Desc
	atlassianUPMUpMetric       *prometheus.Desc
	atlassianUPMPlugins        *prometheus.Desc
	atlassianUPMVersionsMetric *prometheus.Desc
}

// newAtlassianUPMCollector is the constructor for our collector used to initialize the metrics.
func newAtlassianUPMCollector() *atlassianUPMCollector {
	return &atlassianUPMCollector{
		atlassianUPMTimeMetric: prometheus.NewDesc(
			metricNamespace+"_collect_duration_seconds",
			"Used to keep track of how long the Atlassian Universal Plugin Manager (UPM) took to Collect",
			[]string{
				"url",
			},
			nil,
		),
		atlassianUPMUpMetric: prometheus.NewDesc(
			metricNamespace+"_rest_url_up",
			"Used to check if the Atlassian Universal Plugin Manager (UPM) rest endpoint is accessible (https://<app.fqdn>/rest/plugins/1.0/), value is true if up",
			[]string{
				"url",
			},
			nil,
		),
		atlassianUPMPlugins: prometheus.NewDesc(
			metricNamespace+"_plugin",
			"metric used to display plugin information, value is 0",
			[]string{
				"enabled",
				"name",
				"key",
				"installedVersion",
				"userInstalled",
				"url",
			},
			nil,
		),
		atlassianUPMVersionsMetric: prometheus.NewDesc(
			metricNamespace+"_plugin_version_available",
			"Used to monitor the Atlassian Universal Plugin Manager (UPM) versions available, value is true if update available",
			[]string{
				"name",
				"key",
				"availableVersion",
				"installedVersion",
				"enabled",
				"userInstalled",
				"url",
			},
			nil,
		),
	}
}

// Describe is required by prometheus to add our metrics to the default prometheus desc channel
func (collector *atlassianUPMCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.atlassianUPMTimeMetric
	ch <- collector.atlassianUPMUpMetric
	ch <- collector.atlassianUPMPlugins
	ch <- collector.atlassianUPMVersionsMetric
}

// Collect implements required collect function for all prometheus collectors
func (collector *atlassianUPMCollector) Collect(ch chan<- prometheus.Metric) {
	startTime := time.Now()
	log.Debug("Collect start")

	log.Debug("create request object")
	req, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		log.Error("http.NewRequest returned an error:", err)
	}

	log.Debug("create Basic auth string from argument passed")
	bearer = "Basic " + *token

	log.Debug("add authorization header to the request")
	req.Header.Add("Authorization", bearer)

	log.Debug("add content type to the request")
	req.Header.Add("content-type", "application/json")

	log.Debug("make request... get back a response")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Debug("set metric atlassian_upm_rest_url_up")
		ch <- prometheus.MustNewConstMetric(collector.atlassianUPMUpMetric, prometheus.GaugeValue, 0, *fqdn)
		log.Warn("http.DefaultClient.Do returned an error:", err, " return from Collect")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Debug("response status code: ", resp.StatusCode)
	}

	log.Debug("set metric atlassian_upm_rest_url_up")
	ch <- prometheus.MustNewConstMetric(collector.atlassianUPMUpMetric, prometheus.GaugeValue, 1, *fqdn)

	var allPlugins restPlugins
	if resp.StatusCode == 200 {
		log.Debug("get all plugins")
		allPlugins = plugins(resp)

		// return user-installed plugins if argument passed
		if *userInstalled {
			log.Debug("-user-installed found")
			allPlugins = userInstalledPlugins(allPlugins)
		}

		// plugins have the ability to be installed, but disabled, this will remove them if disabled
		if *dropDisabled {
			log.Debug("-drop-disabled found")
			allPlugins = dropDisabledPlugins(allPlugins)
		}

		// Jira specific
		// some plugins maintained by Jira have an additional element, this gives the option to drop those plugins
		if *dropJiraSoftware {
			log.Debug("-drop-jira-software found")
			allPlugins = dropJiraSoftwarePlugins(allPlugins)
		}

		log.Debug("range over values in response, add each as metric with labels")
		for _, plugin := range allPlugins.Plugins {

			log.Debug("creating plugin metric for: " + plugin.Name)
			ch <- prometheus.MustNewConstMetric(
				collector.atlassianUPMPlugins,
				prometheus.GaugeValue,
				0,
				strconv.FormatBool(plugin.Enabled), // convert bool to string for the 'enabled' value in the labels
				string(plugin.Name),
				string(plugin.Key),
				string(plugin.Version),
				strconv.FormatBool(plugin.UserInstalled),
				*fqdn,
			)
		}
	}

	if resp.StatusCode == 200 && *checkUpdates {
		log.Debug("get remaining plugins available info")
		availablePluginsMap := getAvailablePluginInfo(allPlugins)

		log.Debug("range over values in response, add each as metric with labels")
		for _, plugin := range availablePluginsMap {
			availableUpdate := false

			if plugin.InstalledVersion != plugin.Version {
				log.Debug("plugin: ", plugin.Name, ", is currently running: ", plugin.InstalledVersion, ", and can be upgraded to: ", plugin.Version)
				availableUpdate = true
			}

			log.Debug("creating plugin version metric for: ", plugin.Name, ", with Key: ", plugin.Key)
			ch <- prometheus.MustNewConstMetric(
				collector.atlassianUPMVersionsMetric,
				prometheus.GaugeValue,
				boolToFloat(availableUpdate),
				string(plugin.Name),
				string(plugin.Key),
				string(plugin.Version),
				string(plugin.InstalledVersion),
				strconv.FormatBool(plugin.Enabled), // convert bool to string for the 'enabled' value in the labels
				strconv.FormatBool(plugin.UserInstalled),
				*fqdn,
			)
		}
	}

	finishTime := time.Now()
	elapsedTime := finishTime.Sub(startTime)
	log.Debug("set the duration metric")
	ch <- prometheus.MustNewConstMetric(collector.atlassianUPMTimeMetric, prometheus.GaugeValue, elapsedTime.Seconds(), *fqdn)

	log.Debug("Collect finished")
}

// restPlugins structure associated with the rest/plugins/1.0/ endpoint.
// Have dropped most of the response, can check with: curl -s -u peter.gallerani@polarisalpha.com:$PA_PW https://bitbucket.polarisalpha.com/rest/plugins/latest/ | jq '.'"plugins"[0]
type restPlugins struct {
	Plugins []struct {
		Enabled bool `json:"enabled"`
		Links   struct {
			Self          string `json:"self"`
			PluginSummary string `json:"plugin-summary"`
			Modify        string `json:"modify"`
			PluginIcon    string `json:"plugin-icon"`
			PluginLogo    string `json:"plugin-logo"`
			Manage        string `json:"manage"`
		} `json:"links"`
		Name          string `json:"name"`
		Version       string `json:"version"`
		UserInstalled bool   `json:"userInstalled"`
		Optional      bool   `json:"optional"`
		Static        bool   `json:"static"`
		Unloadable    bool   `json:"unloadable"`
		Description   string `json:"description"`
		Key           string `json:"key"`
		UsesLicensing bool   `json:"usesLicensing"`
		Remotable     bool   `json:"remotable"`
		Vendor        struct {
			Name            string `json:"name"`
			MarketplaceLink string `json:"marketplaceLink"`
			Link            string `json:"link"`
		} `json:"vendor"`
		ApplicationKey string `json:"applicationKey,omitempty"` // only found on some jira plugins with this key being "jira-software"
	} `json:"plugins"`
}

// restPluginsAvailable is associated with the UPM /rest/plugins/1.0/available/<key>-key JSON structure returned.
// This is minimal for our uses and can be expanded if needed.
type restPluginsAvailable struct {
	Name             string `json:"name"`
	Version          string `json:"version"`
	InstalledVersion string `json:"installedVersion"`
	Enabled          bool   `json:"enabled"` // doesn't exist in the available json, will copy the enabled value from the restPlugins json
	UserInstalled    bool   `json:"userInstalled"`
	Key              string `json:"key"`
}

// rootHandler accepts calls to "/". This can be used to see if the service is running.
func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, exporterName+" is running")
	log.Debug(r.RemoteAddr, " requested ", r.URL)
}

// faviconHandler responds to /favicon.ico requests.
// This is set to stop error logs from generating when certian browsers that request favicon.ico and the server doesn't have that page.
func faviconHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "")
}

// plugins is used to get all of the plugins from the UPM rest api endpoint response.
func plugins(resp *http.Response) restPlugins {

	log.Debug("get the body out of the response")
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("ioutil.ReadAll returned an error:", err)
	}

	log.Debug("create the json map to unmarshal the json body into")
	var restPluginsMap restPlugins

	log.Debug("unmarshal (turn unicode back into a string) request body into map structure")
	err = json.Unmarshal(body, &restPluginsMap)
	if err != nil {
		log.Error("error Unmarshalling: ", err)
		log.Info("Problem unmarshalling the following string: ", string(body))
	}

	return restPluginsMap
}

// userInstalledPlugins goes over the provided plugins and drops any that are not user-installed.
// During development we have decided to keep plugins that show up for administrators to click the update button, but are not user-installed
func userInstalledPlugins(restPluginsMap restPlugins) restPlugins {
	log.Debug("removing plugins that are not 'user-installed'")
	var tempMap restPlugins
	for _, plugin := range restPluginsMap.Plugins {
		if bool(plugin.UserInstalled) {
			tempMap.Plugins = append(tempMap.Plugins, plugin)
		} else {
			switch plugin.Name {
			case "Atlassian Universal Plugin Manager Plugin":
				tempMap.Plugins = append(tempMap.Plugins, plugin)
			case "Atlassian Troubleshooting and Support Tools":
				tempMap.Plugins = append(tempMap.Plugins, plugin)
			default:
				log.Debug("dropping: ", plugin.Name)
			}
		}
	}
	return tempMap
}

// dropDisabledPlugins goes over the provided plugins and drops any that are not Enabled.
func dropDisabledPlugins(plugins restPlugins) restPlugins {
	log.Debug("removing plugins that are disabled")
	var tempMap restPlugins
	for _, plugin := range plugins.Plugins {
		if bool(plugin.Enabled) {
			tempMap.Plugins = append(tempMap.Plugins, plugin)
		} else {
			log.Debug("dropping: ", plugin.Name)
		}
	}

	return tempMap
}

// dropJiraSoftwarePlugins goes over the provided plugins and drops any that are built by atlassian.
func dropJiraSoftwarePlugins(plugins restPlugins) restPlugins {
	log.Debug("removing plugins that have 'jira-software' for the 'key' value")
	var tempMap restPlugins
	for _, plugin := range plugins.Plugins {
		if plugin.ApplicationKey == "jira-software" {
			log.Debug("dropping: ", plugin.Name)
		} else {
			tempMap.Plugins = append(tempMap.Plugins, plugin)
		}
	}

	return tempMap
}

// getAvailablePluginInfo uses the given map of plugins and gets the available information for that plugin.
// The map returned is an available structure.
func getAvailablePluginInfo(restPluginsMap restPlugins) []restPluginsAvailable {
	var availablePluginsMap []restPluginsAvailable
	for _, plugin := range restPluginsMap.Plugins {
		log.Debug("getting: ", plugin.Name, ", available info")
		availablePluginURL := baseURL + "available/" + plugin.Key + "-key"
		log.Debug("requesting URL: " + availablePluginURL)
		req, err := http.NewRequest("GET", availablePluginURL, nil)
		if err != nil {
			log.Error("http.NewRequest returned an error:", err)
		}

		log.Debug("add authorization header to the request")
		req.Header.Add("Authorization", bearer)

		log.Debug("make request... get back a response")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Error("http.DefaultClient.Do returned an error:", err)
		}
		defer res.Body.Close()

		if res.StatusCode != 200 {
			log.Debug("response status code: ", res.StatusCode, " continuing to next plugin")
			continue
		}

		log.Debug("get the body out of the response")
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Error("ioutil.ReadAll returned an error:", err)
		}

		if len(body) < 1 {
			log.Debug("body was empty, continue to next plugin")
			continue
		}

		log.Debug("create temp map object")
		var tempMap restPluginsAvailable

		log.Debug("unmarshal (turn unicode back into a string) request body into map structure")
		err = json.Unmarshal(body, &tempMap)
		if err != nil {
			log.Error("error Unmarshalling: ", err)
			log.Info("Problem unmarshalling the following string: ", string(body))
		}

		// add the enabled value from the plugin map to the available map
		tempMap.Enabled = plugin.Enabled

		log.Debug("adding plugin: ", tempMap.Name, ", and Key: ", tempMap.Key)
		availablePluginsMap = append(availablePluginsMap, tempMap)

	}

	return availablePluginsMap
}

// boolToFloat converts a boolean value to a float64
func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func main() {
	flag.Parse()

	// Can't log.Debug, because it doesn't exist at the moment

	// check if help has been passed
	if *help {
		usage()
	}

	// check for required arguments
	if *fqdn == "" {
		fmt.Printf("-app.fqdn must be provided\n\n")
		usage()
	}
	if *token == "" {
		fmt.Printf("-app.token must be provided\n\n")
		usage()
	}

	// adjust the logrus logger if arguments passed
	if *enableColLogs {
		disableCol = false
	}
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
		DisableColors: disableCol,
	})

	// check for debug argument
	if *debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("set log level to: debug")
	}

	// Create a new instance of the Collector and then register it with the prometheus client.
	upmCollector := newAtlassianUPMCollector()
	prometheus.MustRegister(upmCollector)

	log.Info("starting...")

	log.Debug("create http server listening at: ", *address, ":", *port)
	srv := http.Server{
		Addr: *address + ":" + *port,
	}

	log.Debug("add / handler")
	http.HandleFunc("/", rootHandler)

	log.Debug("add /favicon.ico handler") // because browsers request /favicon.ico, we add a handler so our metrics don't get false calls
	http.HandleFunc("/favicon.ico", faviconHandler)

	log.Debug("add /metrics handler")
	http.Handle("/metrics", promhttp.Handler())

	log.Debug("set rest plugins url from arguments")
	baseURL = *protocol + "://" + *fqdn + "/rest/plugins/latest/"
	log.Debug("url: ", baseURL)

	log.Debug("make a channel of type os.Signal with a 1 space buffer size")
	ch := make(chan os.Signal, 1)

	// when a SIGNAL of a certain type happens, put it 'on' the channel
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	log.Debug("start the http server in a goroutine (pew -->)")
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatal("ListenAndServe Error:", err)
		}
	}()

	log.Info(exporterName, " is ready to take requests at: ", *address+":"+*port)

	// channels block, so the program will wait (stay running) here till it gets a signal
	s := <-ch
	log.Info("SIGNAL received: ", s)

	close(ch)
	log.Debug("signal channel closed")

	log.Info("shutting down http server...")
	err := srv.Shutdown(context.Background())
	if err != nil {
		// Error from closing listeners, or context timeout
		log.Fatal("Shutdown error: ", err)
	}

	log.Info(exporterName, " was gracefully shutdown")
}

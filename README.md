# Atlassian UPM (Universal Plugin Manager) Exporter for Prometheus

[![Go Report Card](https://goreportcard.com/badge/github.com/polarisalpha/atlassian_upm_exporter)](https://goreportcard.com/report/github.com/polarisalpha/atlassian_upm_exporter)

The Universal Plugin Manager (UPM) is a tool for administering apps in Atlassian applications. You can use the UPM to find, install, manage, and configure apps. The UPM interface is the same across products.

## Docker Build Example

```none
docker build . -t atlassian_upm_exporter
```

## Docker Run Example

List Help

```none
docker run -it --rm atlassian_upm_exporter -help
```

Get all plugins example

```none
docker run -it --rm -p 9996:9996 atlassian_upm_exporter -app.fqdn="<bitbucket|confluence|jira>.domain.com" -app.token='<base64>'
```

Get all 'user-installed' plugins, and check for updates example

```none
docker run -it --rm -p 9996:9996 atlassian_upm_exporter -app.fqdn="<bitbucket|confluence|jira>.domain.com" -app.token='<base64>' -user-installed -check-updates
```

If monitoring Jira, can drop 'jira-software' plugins as well

```none
docker run -it --rm -p 9996:9996 atlassian_upm_exporter -app.fqdn="<bitbucket|confluence|jira>.domain.com" -app.token='<base64>' -user-installed -check-updates -drop-jira-software-plugins
```

## Prometheus Job

```none
- job_name: "atlassian_upm_exporter"
  static_configs:
  - targets:
    - 'host.domain.com:9996'
```

## Questions/Troubleshooting

### Why does it make one request per plugin to check availability?

1. The initial plugin endpoint lists all plugins with the current installed version. It has to get the Key for the plugin, then reach out to check the avilable endpoint, then compare the available update version for each plugin.

### Plugin is 'user-installed' but no update metric and seeing 404s with -debug?

1. The plugin doesn't have any requestable Key data at: `/rest/plugin/latest/availble/<plugin.Key>-key`. I'm not fully sure, but I think this is on the maintainer of the plugin to implement their plugin correctly on the Atlassian Marketplace.

## References

* [https://confluence.atlassian.com/upm/universal-plugin-manager-documentation-273875696.html](https://confluence.atlassian.com/upm/universal-plugin-manager-documentation-273875696.html)
* [https://ecosystem.atlassian.net/wiki/spaces/UPM/pages/6094960/UPM+REST+API](https://ecosystem.atlassian.net/wiki/spaces/UPM/pages/6094960/UPM+REST+API)

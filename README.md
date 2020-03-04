# Atlassian UPM (Universal Plugin Manager) Exporter for Prometheus

The Universal Plugin Manager (UPM) is a tool for administering apps in Atlassian applications. You can use the UPM to find and install, manage, and configure. apps. The UPM interface is the same across products.

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
docker run -it --rm -p 9997:9997 atlassian_upm_exporter -app.fqdn="<bitbucket|confluence|jira>.domain.com" -app.token=''
```

Get all 'user-installed' plugins, and check for updates example

```none
docker run -it --rm -p 9997:9997 atlassian_upm_exporter -app.fqdn="<bitbucket|confluence|jira>.domain.com" -app.token='' -user-installed -check-updates
```

## Prometheus Job

```none
- job_name: "atlassian_upm_exporter"
  static_configs:
  - targets:
    - 'host.domain.com:9997'
```

## References

* [https://confluence.atlassian.com/upm/universal-plugin-manager-documentation-273875696.html](https://confluence.atlassian.com/upm/universal-plugin-manager-documentation-273875696.html)
* [https://ecosystem.atlassian.net/wiki/spaces/UPM/pages/6094960/UPM+REST+API](https://ecosystem.atlassian.net/wiki/spaces/UPM/pages/6094960/UPM+REST+API)

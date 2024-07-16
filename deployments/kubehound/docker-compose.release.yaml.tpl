name: kubehound-release
services:
  mongodb:
    ports:
      - "127.0.0.1:27017:27017"

  kubegraph:
    image: ghcr.io/datadog/kubehound-graph:{{ .VersionTag }}
    ports:
      - "127.0.0.1:8182:8182"
      - "127.0.0.1:8099:8099"
  
  ui-jupyter:
    image: ghcr.io/datadog/kubehound-ui:{{ .VersionTag }}

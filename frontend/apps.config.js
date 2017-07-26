const CONFIG = {
  apps: [{
    "name": "ui",
    "title": "Datamesh"
  }],
  apiServers: [{
    host: process.env.API_SERVICE_HOST || 'datamesh-server',
    port: process.env.API_SERVICE_PORT || 6969
  }]
}

module.exports = CONFIG
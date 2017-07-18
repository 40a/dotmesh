import 'babel-polyfill'
import path from 'path'
import express from 'express'
import webpack from 'webpack'
import morgan from 'morgan'
import fs from 'fs'
import httpProxy from 'http-proxy'
import webpackDevMiddleware from 'webpack-dev-middleware'
import webpackHotMiddleware from 'webpack-hot-middleware'

import config from './webpack.config'
import appConfig from './app.config'

const APPS = appConfig.apps
const app = express()
const compiler = webpack(config)

const devMiddleware = webpackDevMiddleware(compiler, {
  noInfo: true,
  publicPath: config.output.publicPath,
  stats: {
    colors: true
  }
})

app.use(morgan('tiny'))
app.use(webpackHotMiddleware(compiler, {
  log: console.log
}))
app.use(devMiddleware)

// catch all route which serves cached webpack html (with hashed paths)
const appServer = (appConfig) => (req, res) => {
  const htmlBuffer = devMiddleware.fileSystem.readFileSync(`${config.output.path}/${appConfig.name}/index.html`)
  res.send(htmlBuffer.toString())
}

APPS.forEach(appConfig => {
  if(!appConfig.name) throw new Error('each app must have a name')
  const route = `/${appConfig.name}*`
  console.log(`mounting ${route}`)
  app.get(route, appServer(appConfig))
})

app.listen(process.env.PORT || 80, '0.0.0.0', function (err) {
  if (err) {
    console.log(err);
    return;
  }
})
'use strict'

const assert = require('assert')

const screenshot = (browser, path) => {
  browser.saveScreenshot('screenshots' + path)
}

const checkUrl = (browser, path, message) => {
  browser.url(result => {
    const checkUrl = (browser.launchUrl + path).replace(':80', '')
    console.log(`checking URL: ${checkUrl} vs ${result.value}`)
    assert(result.value == checkUrl, message)
  })
}

module.exports = {
  screenshot,
  checkUrl
}

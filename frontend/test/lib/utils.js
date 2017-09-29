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

// switch between the application (0) and the terminal (1)
const switchWindow = (browser, windowIndex) => {
  browser.window_handles(function(result) {
    var handle = result.value[windowIndex]
    browser.switchWindow(handle)
  })
}
const switchApplication = (browser) => switchWindow(browser, 0)
const switchTerminal = (browser) => switchWindow(browser, 1)

// open the gotty terminal based on the env
const launchTerminal = (browser) => {
  browser
    .execute(function (url) {
      window.open(url, '_blank')
    }, [`http://${process.env.GOTTY_HOST}`]);
}

const typeTerminal = (browser, command) => {
  browser.keys(command)
  browser.keys(browser.Keys.ENTER) 
}

const possible = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";

const randomChars = (len) => {
  len = len || 5
  let text = "";

  for (var i = 0; i < len; i++) {
    text += possible.charAt(Math.floor(Math.random() * possible.length))
  }
  return text;
}

const checkElementCount = (browser, selector, expected) => {
  browser.elements('css selector', selector, (result) => {
    const len = result.value.length
    console.log(`checking Element count: ${selector} = ${len} vs ${expected}`)
    assert(len == expected, `checking Element count: ${selector} = ${len} vs ${expected}`)
  })
}

module.exports = {
  screenshot,
  checkUrl,
  switchApplication,
  switchTerminal,
  launchTerminal,
  typeTerminal,
  randomChars,
  checkElementCount
}

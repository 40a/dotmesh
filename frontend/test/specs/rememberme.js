'use strict'

const assert = require('assert')
const utils = require('../lib/utils')

module.exports = {
  loadLoginPage: (browser) => {
    const home = browser.page.home().navigate()
    home.waitForElementVisible('@loginLink', 2000)
    home.click('@loginLink')
    browser.pause(300)
  },
  login: (browser) => {
    const login = browser.page.login()
    const volumes = browser.page.volumes()
    login.setValue('@username', process.env.TEST_USER)
    login.setValue('@password', process.env.TEST_PASSWORD)
    browser.pause(300)
    login.click('@submitButton')
    volumes.waitForElementVisible('@title', 2000)
    utils.checkUrl(browser, '/dashboard', 'the page is now on dashboard and logged in')    
  },
  refresh: (browser) => {
     browser
        .refresh()
    browser.pause(3000)
    browser.end()
  }
}
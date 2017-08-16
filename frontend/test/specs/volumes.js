'use strict'

const assert = require('assert')
const utils = require('../lib/utils')

module.exports = {
  login: (browser) => {
    const home = browser.page.home().navigate()
    const login = browser.page.login()
    const volumes = browser.page.volumes()
    home.waitForElementVisible('@title', 2000)
    home.expect.element('@title').text.to.equal('Datamesh Console')
    home.click('@loginLink')
    browser.pause(1000)
    login.setValue('@username', process.env.TEST_USER)
    login.setValue('@password', process.env.TEST_PASSWORD)
    browser.pause(1000)
    login.click('@submitButton')
    volumes.waitForElementVisible('@title', 2000)
    utils.checkUrl(browser, '/dashboard', 'the page is now on dashboard and logged in')
    browser.end()
  }
}
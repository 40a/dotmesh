'use strict'

const assert = require('assert')

const screenshot = (browser, path) => {
  browser.saveScreenshot('screenshots' + path)
}

module.exports = {
  loadHomepage: (browser) => {
    const home = browser.page.home().navigate()
    home.waitForElementVisible('@title', 2000)
    home.expect.element('@title').text.to.equal('Datamesh Console')
    browser.pause(300)
    screenshot(browser, '/homepage.png')
  },
  visitRegisterPage: (browser) => {
    const home = browser.page.home()
    const register = browser.page.register()
    home.click('@registerLink')
    register.waitForElementVisible('@title', 2000)
    register.expect.element('@title').text.to.equal('Register')
    browser.pause(300)
    screenshot(browser, '/registerPage.png')
  },
  invalidRegisterValues: (browser) => {
    const register = browser.page.register()
    register.setValue('@email', 't')
    register.click('@username')
    browser.pause(300)
    register.expect.element('@emailError').text.to.equal('Invalid email address')
    register.click('@password')
    browser.pause(300)
    register.expect.element('@usernameError').text.to.equal('Required')
    register.click('@email')
    browser.pause(300)
    register.expect.element('@passwordError').text.to.equal('Required')
    screenshot(browser, '/registerPageInvalid.png')
    register.click('@submitButton')
    browser.pause(300)
    browser.url(result => {
      const checkUrl = (browser.launchUrl + '/register').replace(':80', '')
      console.log(`checking URL: ${checkUrl} vs ${result.value}`)
      assert(result.value == checkUrl, 'the page is still on register')
    })
  },
  correctRegisterValues: (browser) => {
    const register = browser.page.register()
    register.setValue('@email', process.env.TEST_EMAIL)
    register.setValue('@username', process.env.TEST_USER)
    register.setValue('@password', process.env.TEST_PASSWORD)
    browser.pause(300)
    register.expect.element('@emailError').to.not.be.present
    register.expect.element('@usernameError').to.not.be.present
    register.expect.element('@passwordError').to.not.be.present
    screenshot(browser, '/registerPageValid.png')
  },
  submitForm: (browser) => {
    const register = browser.page.register()
    const volumes = browser.page.volumes()
    register.click('@submitButton')
    volumes.waitForElementVisible('@title', 2000)
    browser.url(result => {
      const checkUrl = (browser.launchUrl + '/dashboard').replace(':80', '')
      console.log(`checking URL: ${checkUrl} vs ${result.value}`)
      assert(result.value == checkUrl, 'the page is now on register and logged in')
    })
    screenshot(browser, '/postRegisterDashboard.png')
  },
  logout: (browser) => {
    const home = browser.page.home()
    home.click('@rightMenuButton')
    browser.pause(1000)
    screenshot(browser, '/rightMenuDropdown.png')
    home.click('@logoutButton')
    browser.pause(1000)
    browser.url(result => {
      const checkUrl = (browser.launchUrl + '/login').replace(':80', '')
      console.log(`checking URL: ${checkUrl} vs ${result.value}`)
      assert(result.value == checkUrl, 'the page is now on the login screen')
    })
    screenshot(browser, '/postLogout.png')
  }
}
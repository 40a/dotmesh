'use strict'

const assert = require('assert')
const utils = require('../lib/utils')

const TIMESTAMP = utils.randomChars()

module.exports = {
  loadHomepage: (browser) => {
    const home = browser.page.home().navigate()
    home.waitForElementVisible('@title', 2000)
    home.expect.element('@title').text.to.equal('Datamesh Cloud')
    browser.pause(300)
    utils.screenshot(browser, '/homepage.png')
  },
  visitRegisterPage: (browser) => {
    const home = browser.page.home()
    const register = browser.page.register()
    home.click('@registerLink')
    register.waitForElementVisible('@title', 2000)
    register.expect.element('@title').text.to.equal('Register')
    browser.pause(300)
    utils.screenshot(browser, '/registerPage.png')
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
    utils.screenshot(browser, '/registerPageInvalid.png')
    register.click('@submitButton')
    browser.pause(300)
    utils.checkUrl(browser, '/register', 'the page is still on register')
  },
  correctRegisterValues: (browser) => {
    const register = browser.page.register()
    register.setValue('@email', `user${TIMESTAMP}@user.com`)
    register.setValue('@username', `user${TIMESTAMP}`)
    register.setValue('@password', `password`)
    browser.pause(300)
    register.expect.element('@emailError').to.not.be.present
    register.expect.element('@usernameError').to.not.be.present
    register.expect.element('@passwordError').to.not.be.present
    utils.screenshot(browser, '/registerPageValid.png')
  },
  submitRegisterForm: (browser) => {
    const register = browser.page.register()
    const userlayout = browser.page.userlayout()
    register.click('@submitButton')
    userlayout.waitForElementVisible('@gravatar', 2000)
    utils.checkUrl(browser, '/dashboard', 'the page is now on dashboard and logged in')
    utils.screenshot(browser, '/postRegisterDashboard.png')
  },
  logout: (browser) => {
    const home = browser.page.home()
    home.click('@rightMenuButton')
    browser.pause(1000)
    utils.screenshot(browser, '/rightMenuDropdown.png')
    home.click('@logoutButton')
    browser.pause(1000)
    utils.checkUrl(browser, '/login', 'the page is now on the login screen')
    utils.screenshot(browser, '/postLogout.png')
  },
  invalidLoginValues: (browser) => {
    utils.screenshot(browser, '/loginPage.png')
    const login = browser.page.login()
    login.click('@username')
    browser.pause(300)
    login.click('@password')
    browser.pause(300)
    login.click('@username')
    browser.pause(300)
    login.expect.element('@usernameError').text.to.equal('Required')
    login.expect.element('@passwordError').text.to.equal('Required')
    utils.screenshot(browser, '/loginPageInvalid.png')
    login.click('@submitButton')
    browser.pause(300)
    utils.checkUrl(browser, '/login', 'the page is still on the login screen')
  },
  correctLoginValues: (browser) => {
    const login = browser.page.login()
    login.setValue('@username', `user${TIMESTAMP}`)
    login.setValue('@password', `password`)
    browser.pause(300)
    login.expect.element('@usernameError').to.not.be.present
    login.expect.element('@passwordError').to.not.be.present
    utils.screenshot(browser, '/loginPageValid.png')
  },
  submitLoginForm: (browser) => {
    const login = browser.page.login()    
    const userlayout = browser.page.userlayout()
    login.click('@submitButton')
    userlayout.waitForElementVisible('@gravatar', 2000)
    utils.checkUrl(browser, '/dashboard', 'the page is now on dashboard and logged in')
    utils.screenshot(browser, '/postLoginDashboard.png')
  },
  refresh: (browser) => {
     browser
        .refresh()
    browser.pause(3000)
    utils.checkUrl(browser, '/dashboard', 'the page is now on dashboard and logged in')
    browser.pause(1000)
    browser.end()
  }
}

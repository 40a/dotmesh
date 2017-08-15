'use strict'

module.exports = {
  loadHomepage: (browser) => {
    const home = browser.page.home().navigate()
    home.waitForElementVisible('@title', 2000)
    home.expect.element('@title').text.to.equal('Datamesh Console')
    browser.pause(1000)
  },
  visitRegisterPage: (browser) => {
    const home = browser.page.home()
    const register = browser.page.register()
    home.click('@registerLink')
    register.waitForElementVisible('@title', 2000)
    register.expect.element('@title').text.to.equal('Register')
    browser.pause(1000)
  },
  invalidValues: (browser) => {
    const register = browser.page.register()
    register.setValue('@email', 't')
    register.click('@username')
    browser.pause(1000)
    register.expect.element('@emailError').text.to.equal('Invalid email address')
    register.click('@password')
    browser.pause(1000)
    register.expect.element('@usernameError').text.to.equal('Required')
    register.click('@email')
    browser.pause(1000)
    register.expect.element('@passwordError').text.to.equal('Required')
    browser.end()
  }
}
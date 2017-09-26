'use strict'

const assert = require('assert')
const utils = require('../lib/utils')

const TIMESTAMP = utils.randomChars()

module.exports = {

  // register alice and check we get the dashboard with help
  endToEndTest: (browser) => {

    // pages
    const home = browser.page.home()
    const register = browser.page.register()
    const userlayout = browser.page.userlayout()
    const repos = browser.page.repos()
    const repoform = browser.page.repoform()

    // open app
    home.navigate()

    // wait for loading and click register link
    home.waitForElementVisible('@title', 2000)
    home.expect.element('@title').text.to.equal('Datamesh Console')
    browser.pause(2000)
    home.click('@registerLink')

    /*
    
        ALICE
        
    */

    // wait for loading and fill register form then click submit
    register.waitForElementVisible('@title', 2000)
    register.expect.element('@title').text.to.equal('Register')
    browser.pause(2000)
    register.setValue('@email', `alice${TIMESTAMP}@alice.com`)
    register.setValue('@username', `alice${TIMESTAMP}`)
    register.setValue('@password', `password`)
    browser.pause(2000)
    register.click('@submitButton')

    // wait for avatar and username to load
    userlayout.waitForElementVisible('@gravatar', 2000)
    userlayout.expect.element('@username').text.to.equal(`alice${TIMESTAMP}`)
    browser.pause(2000)
    // click the new buton and wait for form to be visible
    repos.click('@newbutton')
    repoform.waitForElementVisible('@name', 2000)
    browser.pause(2000)

    // enter the new repo name and submit the form
    repoform.setValue('@name', `apples`)
    browser.pause(2000)
    repoform.click('@submitButton')
    repos.waitForElementVisible('@container', 2000)
    browser.pause(2000)
    utils.checkElementCount(browser, '#repo-list-container #repo-list-item', 1)
  
    // open terminal
    utils.launchTerminal(browser)
    utils.switchTerminal(browser)
    browser.pause(2000)

    // add alice remote
    utils.typeTerminal(browser, `DATAMESH_PASSWORD=password dm remote add alice${TIMESTAMP} alice${TIMESTAMP}@localhost`)
    browser.pause(2000)
    utils.typeTerminal(browser, `dm remote switch alice${TIMESTAMP}`)
    browser.pause(2000)
    utils.typeTerminal(browser, `dm list`)
    browser.pause(2000)

    // add oranges volume from CLI
    utils.typeTerminal(browser, `dm init alice${TIMESTAMP}/oranges`)
    browser.pause(2000)
    utils.typeTerminal(browser, `dm list`)
    browser.pause(2000)

    // switch to the application - open the form can cancel to get a reloaded repo list
    // (TODO: disable the 10 second dev delay when running tests)
    utils.switchApplication(browser)
    repos.click('@newbutton')
    repoform.waitForElementVisible('@name', 2000)
    repoform.click('@cancelButton')
    repos.waitForElementVisible('@container', 2000)
    browser.pause(2000)
    utils.checkElementCount(browser, '#repo-list-container #repo-list-item', 2)

    // try some searches and check the result count
    repos.setValue('@search', `oranges`)
    browser.pause(500)
    utils.checkElementCount(browser, '#repo-list-container #repo-list-item', 1)
    repos.clearValue('@search')
    repos.setValue('@search', `oranges2`)
    browser.pause(500)
    utils.checkElementCount(browser, '#repo-list-container #repo-list-item', 0)
    repos.clearValue('@search')
    repos.setValue('@search', `apples`)
    browser.pause(500)
    utils.checkElementCount(browser, '#repo-list-container #repo-list-item', 1)
    repos.clearValue('@search')
    repos.setValue('@search', ` `)
    browser.pause(500)
    utils.checkElementCount(browser, '#repo-list-container #repo-list-item', 2)


    browser.end()
  },
}
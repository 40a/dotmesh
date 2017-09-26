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
    const login = browser.page.login()
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

    // logout
    home.click('@rightMenuButton')
    browser.pause(500)
    home.click('@logoutButton')
    browser.pause(1000)
    utils.checkUrl(browser, '/login', 'the page is now on the login screen')

    /*
    
        BOB
        
    */

    home.navigate()
    // wait for loading and click register link
    home.waitForElementVisible('@title', 2000)
    home.expect.element('@title').text.to.equal('Datamesh Console')
    browser.pause(2000)
    home.click('@registerLink')

    // wait for loading and fill register form then click submit
    register.waitForElementVisible('@title', 2000)
    register.expect.element('@title').text.to.equal('Register')
    browser.pause(2000)
    register.setValue('@email', `bob${TIMESTAMP}@bob.com`)
    register.setValue('@username', `bob${TIMESTAMP}`)
    register.setValue('@password', `password`)
    browser.pause(2000)
    register.click('@submitButton')

    // wait for avatar and username to load
    userlayout.waitForElementVisible('@gravatar', 2000)
    userlayout.expect.element('@username').text.to.equal(`bob${TIMESTAMP}`)
    browser.pause(2000)

    utils.switchTerminal(browser)
    browser.pause(2000)

    // add bob remote
    utils.typeTerminal(browser, `DATAMESH_PASSWORD=password dm remote add bob${TIMESTAMP} bob${TIMESTAMP}@localhost`)
    browser.pause(2000)
    utils.typeTerminal(browser, `dm remote switch bob${TIMESTAMP}`)
    browser.pause(2000)
    utils.typeTerminal(browser, `dm list`)
    browser.pause(2000)
    utils.typeTerminal(browser, `dm init bob${TIMESTAMP}/pears`)
    browser.pause(2000)
    utils.typeTerminal(browser, `dm list`)
    browser.pause(2000)

    // switch to browser
    utils.switchApplication(browser)
    repos.click('@newbutton')
    repoform.waitForElementVisible('@name', 2000)
    repoform.click('@cancelButton')
    repos.waitForElementVisible('@container', 2000)
    browser.pause(2000)
    utils.checkElementCount(browser, '#repo-list-container #repo-list-item', 1)

    // TODO - the pages thing falls down from here as we sketch this test out
    // TODO - refactor the following into pages like above

    // click the volume we just created
    browser.click('#repo-list-container #repo-list-item:nth-child(1) #repo-list-item-link')
    browser.pause(1000)
    utils.checkUrl(browser, `/bob${TIMESTAMP}/pears`, 'the page is now on the pears volume screen')
    browser.pause(1000)

    // switch to the settings page
    browser.click('div[data-react-toolbox=tab]:nth-child(2)')
    browser.pause(1000)
    utils.checkUrl(browser, `/bob${TIMESTAMP}/pears/settings`, 'the page is now on the pears volume screen')
    browser.pause(1000)

    // add alice as a collaborator
    browser.click('div[data-react-toolbox=tab]:nth-child(2)')
    browser.setValue('#collaborator-search input[name=search]', `alice${TIMESTAMP}`)
    browser.pause(1000)
    browser.click('#collaborator-buttons button#add')
    browser.pause(1000)

    // logout
    home.click('@rightMenuButton')
    browser.pause(500)
    home.click('@logoutButton')
    browser.pause(1000)
    utils.checkUrl(browser, '/login', 'the page is now on the login screen')

    /*
    
        ALICE
        
    */

    login.setValue('@username', `alice${TIMESTAMP}`)
    login.setValue('@password', `password`)
    login.click('@submitButton')
    repos.waitForElementVisible('@container', 2000)
    browser.pause(2000)
    utils.checkElementCount(browser, '#repo-list-container #repo-list-item', 3)
    repos.clearValue('@search')
    repos.setValue('@search', `pears`)
    browser.pause(2000)
    utils.checkElementCount(browser, '#repo-list-container #repo-list-item', 1)

    utils.switchTerminal(browser)
    browser.pause(2000)
    utils.typeTerminal(browser, `dm remote switch alice${TIMESTAMP}`)
    browser.pause(2000)
    utils.typeTerminal(browser, `dm list`)
    browser.pause(2000)

    browser.end()
  },
}
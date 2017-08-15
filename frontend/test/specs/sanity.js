'use strict'

module.exports = {
  getTitlePass: (browser) => {
    const home = browser.page.home().navigate()
    home.waitForElementVisible('@title', 2000)
    home.expect.element('@title').text.to.equal('Datamesh Console')
  }
}
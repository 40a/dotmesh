'use strict'

module.exports = {
  getTitlePass: (client) => {
    const home = client.page.home()
    home.navigate().waitForElementVisible('body', 2000)
    //calendar.expect.element('@title').to.be.visible
    //calendar.expect.element('@title').text.to.equal('Datamesh Console')
  }
}
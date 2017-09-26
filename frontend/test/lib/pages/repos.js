'use strict'

module.exports = {
  url: function() {
    return this.api.launchUrl
  },
  elements: {
    page: {
      selector: '#userLayout'
    }
  }
}
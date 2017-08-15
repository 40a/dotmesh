'use strict'

module.exports = {
  url: function() {
    return this.api.launchUrl
  },
  elements: {
    title: {
      selector: '#volumePage #title'
    }
  }
}
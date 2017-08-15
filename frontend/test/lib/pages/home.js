'use strict'

module.exports = {
  url: function() {
    return this.api.launchUrl + '/ui'
  },
  elements: {
    title: {
      selector: 'header #appBarTitle'
    }
  }
}
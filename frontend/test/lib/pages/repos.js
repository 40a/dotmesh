'use strict'

module.exports = {
  url: function() {
    return this.api.launchUrl
  },
  elements: {
    newbutton: {
      selector: '#repo-list-buttons button#new'
    },
    container: {
      selector: '#repo-list-container'
    }
  }
}
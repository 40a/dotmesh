'use strict'

module.exports = {
  url: function() {
    return this.api.launchUrl
  },
  elements: {
    gravatar: {
      selector: '#gravatar-img'
    },
    username: {
      selector: '#user-name-label h2'
    }
  }
}
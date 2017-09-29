'use strict'

module.exports = {
  url: function() {
    return this.api.launchUrl
  },
  elements: {
    name: {
      selector: '#repo-form input[name=Name]'
    },
    cancelButton: {
      selector: '#repo-form button:nth-child(1)'
    },
    submitButton: {
      selector: '#repo-form button:nth-child(2)'
    }
  }
}
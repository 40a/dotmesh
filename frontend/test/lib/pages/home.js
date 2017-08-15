'use strict'

module.exports = {
  url: function() {
    return this.api.launchUrl
  },
  elements: {
    title: {
      selector: 'header #appBarTitle'
    },
    registerLink: {
      selector: '.homePageRegisterLink'
    },
    loginLink: {
      selector: '.homePageLoginLink'
    },
    rightMenuButton: {
      selector: '#appBarMenu button'
    },
    logoutButton: {
      selector: '#appBarMenu ul li:last-child'
    }
  }
}
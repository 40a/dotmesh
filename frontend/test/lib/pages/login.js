'use strict'

module.exports = {
  url: function() {
    return this.api.launchUrl
  },
  elements: {
    title: {
      selector: '.LoginForm #title'
    },
    username: {
      selector: '.LoginForm input[name=username]'
    },
    usernameError: {
      selector: '.LoginForm input[name=username] ~ span:last-child'
    },
    password: {
      selector: '.LoginForm input[name=password]'
    },
    passwordError: {
      selector: '.LoginForm input[name=password] ~ span:last-child'
    },
    submitButton: {
      selector: '.LoginForm button'
    }
  }
}
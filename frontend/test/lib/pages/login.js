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
      selector: '.LoginForm input[name=Name]'
    },
    usernameError: {
      selector: '.LoginForm input[name=Name] ~ span:last-child'
    },
    password: {
      selector: '.LoginForm input[name=Password]'
    },
    passwordError: {
      selector: '.LoginForm input[name=Password] ~ span:last-child'
    },
    submitButton: {
      selector: '.LoginForm button'
    }
  }
}
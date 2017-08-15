'use strict'

module.exports = {
  url: function() {
    return this.api.launchUrl + '/ui'
  },
  elements: {
    title: {
      selector: '.RegisterForm #title'
    },
    email: {
      selector: '.RegisterForm input[name=email]'
    },
    emailError: {
      selector: '.RegisterForm input[name=email] ~ span:last-child'
    },
    username: {
      selector: '.RegisterForm input[name=username]'
    },
    usernameError: {
      selector: '.RegisterForm input[name=username] ~ span:last-child'
    },
    password: {
      selector: '.RegisterForm input[name=password]'
    },
    passwordError: {
      selector: '.RegisterForm input[name=password] ~ span:last-child'
    },
    submitButton: {
      selector: '.RegisterForm button'
    }
  }
}
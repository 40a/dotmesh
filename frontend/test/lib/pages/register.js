'use strict'

module.exports = {
  url: function() {
    return this.api.launchUrl
  },
  elements: {
    title: {
      selector: '.RegisterForm #title'
    },
    email: {
      selector: '.RegisterForm input[name=Email]'
    },
    emailError: {
      selector: '.RegisterForm input[name=Email] ~ span:last-child'
    },
    username: {
      selector: '.RegisterForm input[name=Name]'
    },
    usernameError: {
      selector: '.RegisterForm input[name=Name] ~ span:last-child'
    },
    password: {
      selector: '.RegisterForm input[name=Password]'
    },
    passwordError: {
      selector: '.RegisterForm input[name=Password] ~ span:last-child'
    },
    submitButton: {
      selector: '.RegisterForm button'
    }
  }
}
'use strict'
const video = require('./video')

module.exports = {
  beforeEach: function (browser, done) {
    video.start(browser, done)
  },
  afterEach: function (browser, done) {
    video.stop(browser, done)
  }
}

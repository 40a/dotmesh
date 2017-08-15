'use strict'

module.exports = {
  url: function() {
    console.log('-------------------------------------------');
    console.log('-------------------------------------------');
    console.dir(this.api.launchUrl + '/ui')
    return this.api.launchUrl + '/ui'
  },
  elements: {
    title: {
      selector: 'header #appBarTitle'
    }
  }
}
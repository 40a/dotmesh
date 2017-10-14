import { take, put, call, fork, select, all, takeLatest, takeEvery, cancel } from 'redux-saga/effects'

const GoogleAnalyticsSagas = (opts = {}) => {
  
  function* initialize() {

  }

  function* setUser(user) {
    ga('set', 'userId', user.Id)
  }

  function* visitPage(path, data) {
    ga('send', {
      hitType: 'pageview',
      page: path
    })
  }

  return {
    setUser,
    visitPage,
    initialize
  }
}

export default GoogleAnalyticsSagas

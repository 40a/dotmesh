import { take, put, call, fork, select, all, takeLatest, takeEvery, cancel } from 'redux-saga/effects'

const MixPanelSagas = (opts = {}) => {
  
  function* initialize() {
    mixpanel.track("initialize")
  }

  function* setUser(user) {
    mixpanel.track("user login")
  }

  function* visitPage(path, data) {
    mixpanel.track("change page")
  }

  return {
    setUser,
    visitPage,
    initialize
  }
}

export default MixPanelSagas

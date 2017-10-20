import { take, put, call, fork, select, all, takeLatest, takeEvery, cancel } from 'redux-saga/effects'

const FullStorySagas = (opts = {}) => {
  
  function* initialize() {

  }

  function* setUser(user) {
    FS.identify(user.Id, {
      displayName: user.Name,
      email: user.Email,
    });
  }

  function* visitPage(path, data) {

  }

  return {
    setUser,
    visitPage,
    initialize
  }
}

export default FullStorySagas

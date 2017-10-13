import { take, put, call, fork, select, all, takeLatest, takeEvery, cancel } from 'redux-saga/effects'

const IntercomSagas = (opts = {}) => {
  
  function* initialize() {
    window.Intercom("boot", {
      app_id: "i6t0axbd"
    });
  }

  function* setUser(user) {
    Intercom('update', {"name": user.Name, "email": user.Email, "id": user.Id})
  }

  function* visitPage(path, data) {
    Intercom('update')
  }

  return {
    setUser,
    visitPage,
    initialize
  }
}

export default IntercomSagas

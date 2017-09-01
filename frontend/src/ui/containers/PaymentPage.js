import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import * as selectors from '../selectors'
import * as actions from '../actions'

import PaymentPage from '../components/PaymentPage'

class PaymentPageContainer extends Component {
  render() {
    return (
      <PaymentPage {...this.props} />
    )
  }
}

export default connect(
  (state, ownProps) => ({
    config: selectors.valueSelector(state, 'config'),
    email: selectors.auth.user(state).email
  }),
  (dispatch) => ({
    onToken: (token) => {
      console.log('HAVE TOKEN')
      console.dir(token)
      //dispatch(actions.router.hook('paymentToken', token))
    }
  })
)(PaymentPageContainer)

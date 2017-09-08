import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import * as selectors from '../selectors'
import * as actions from '../actions'

import PaymentPage from '../components/PaymentPage'

class PaymentPageContainer extends Component {
  render() {
    return this.props.stripeKey ? (
      <PaymentPage {...this.props} />
    ) : null
  }
}

export default connect(
  (state, ownProps) => {
    const plan = selectors.billing.planById(state, 'developer') || {}
    const amount = plan.PriceUSD
    const currency = 'USD'
    const email = selectors.auth.email(state)
    const stripeKey = selectors.billing.stripeKey(state)
    return {
      config: selectors.valueSelector(state, 'config'),
      email: selectors.auth.user(state).email,
      plan,
      amount,
      currency,
      email,
      stripeKey
    }
  },
  (dispatch) => ({
    onToken: (token) => {
      console.log('HAVE TOKEN')
      console.dir(token)
      //dispatch(actions.router.hook('paymentToken', token))
    }
  })
)(PaymentPageContainer)

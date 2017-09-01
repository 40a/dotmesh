import React, { Component, PropTypes } from 'react'
import priceUtils from 'template-tools/src/utils/price'
import StripeButton from 'template-ui/lib/components/StripeButton'

class PaymentPage extends Component {
  render() {
    return (
      <div id="paymentPage">
        <StripeButton
          name={ this.props.config.name }
          description={ this.props.config.description }
          panelLabel={ this.props.config.panelLabel }
          stripeKey={ this.props.config.stripeKey }
          amount={ this.props.amount }
          email={ this.props.email }
          onToken={ this.props.onToken }
        />
      </div>
    )
  }
}

export default ServerTable

import React, { Component, PropTypes } from 'react'
import priceUtils from 'template-tools/src/utils/price'
import StripeButton from 'template-ui/lib/components/StripeButton'

class PaymentPage extends Component {
  render() {
    return (
      <div id="paymentPage">
        <StripeButton
          name="Datamesh Developer Plan"
          description="Payment for Datamesh Cloud"
          panelLabel="Rock on!"
          stripeKey={ this.props.config.StripePublicKey }
          amount={ 10000 }
          email={ this.props.email }
          onToken={ this.props.onToken }
        />
      </div>
    )
  }
}

export default PaymentPage 

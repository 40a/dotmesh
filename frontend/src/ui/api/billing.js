const submitPayment = (payload) => ({
  method: 'SubmitPayment',
  params: {
    PaymentDeets: {
      Token: payload.token,
      Plan: payload.plan
    }
  }
})

const BillingApi = {
  submitPayment
}

export default BillingApi
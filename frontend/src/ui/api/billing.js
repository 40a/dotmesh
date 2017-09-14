const submitPayment = (payload) => ({
  method: 'SubmitPayment',
  params: {
    Token: payload.token,
    PlanId: payload.plan
  }
})

const BillingApi = {
  submitPayment
}

export default BillingApi
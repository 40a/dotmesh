import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'
import * as selectors from '../selectors'
import * as actions from '../actions'
import Gravatar from '../components/Gravatar'

class GravatarContainer extends Component {
  render() {
    return (
      <Gravatar {...this.props} />
    )
  }
}

export default connect(
  (state, ownProps) => ({
    emailHash: selectors.auth.emailHash(state),
    name: selectors.auth.name(state)
  }),
  (dispatch) => ({
    
  })
)(GravatarContainer)
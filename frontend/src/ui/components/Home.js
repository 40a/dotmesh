import React, { Component, PropTypes } from 'react'
import { Grid, Row, Col } from 'react-flexbox-grid'

import LinkButton from '../containers/LinkButton'
import spacing from './theme/spacing.css'

class Home extends Component {
  render() {
    return (
      <Grid fluid>
        <Row>
          <Col xs={12}>
            <p>Welcome to DataMesh!</p>
          </Col>
          <Col xs={12} className={ spacing.marginTop }>
            <LinkButton
              label='Register'
              primary
              raised
              url='/register'
            />
          </Col>
          <Col xs={12} className={ spacing.marginTop }>
            <LinkButton
              label='Login'
              primary
              raised
              url='/login'
            />
          </Col>
        </Row>
      </Grid>
    )
  }
}

export default Home
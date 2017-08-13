import React, { Component, PropTypes } from 'react'
import { Grid, Row, Col } from 'react-flexbox-grid'

import LinkButton from '../containers/LinkButton'

class Home extends Component {
  render() {
    return (
      <Grid fluid>
        <Row>
          <Col xs={12} sm={6}>
            <p>Welcome to DataMesh!</p>
          </Col>
          <Col xs={12} sm={6}>
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
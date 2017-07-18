import React, { Component, PropTypes } from 'react'
import { Grid, Row, Col } from 'react-flexbox-grid'

class Home extends Component {
  render() {
    return (
      <Grid fluid>
        <Row>
          <Col lg={12}>
            <div>
              Welcome! this is the dashboard
            </div>
          </Col>
        </Row>
      </Grid>
    )
  }
}

export default Home
import React, { Component, PropTypes } from 'react'
import { Grid, Row, Col } from 'react-flexbox-grid'

class Help extends Component {
  render() {
    return (
      <Grid fluid>
        <Row>
          <Col lg={12}>
            <div>
              Help page
            </div>
          </Col>
        </Row>
      </Grid>
    )
  }
}

export default Help
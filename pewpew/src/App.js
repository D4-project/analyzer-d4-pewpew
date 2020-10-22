/* global window */
import React, { Component } from 'react';
import DeckGL, { GeoJsonLayer, ArcLayer, MapController } from 'deck.gl';
import { Nav, Navbar, NavDropdown } from 'react-bootstrap';
import ReactModal from 'react-modal';

const COUNTRIES = window.location + 'map/ne_50m_admin_0_scale_rank.geojson';
const DAILY = window.location + "daily.json";
const D4 = [];

const MODALSTYLE = {
  overlay: {
    // backgroundColor: 'papayawhip'
    position: 'absolute',
    top: '56px',
    bottom: '40px'
  },
  content: {
    position: 'absolute',
    top: '40px',
    left: '40px',
    right: '40px',
    bottom: '40px'
  }
};

ReactModal.setAppElement('#map');

export default class App extends Component {
  constructor(props) {
    super(props);
    this.state = {
      // deck.gl related
      viewport: {
        width: window.innerWidth,
        height: window.innerHeight,
        longitude: 6.1319346,
        latitude: 49.611621,
        zoom: 4,
        maxZoom: 12
      },
      points: [],
      // modal related
      showModalAbout: false,
      showModalControls: false
    };
    // deck.gl related
    this._getData = this._getData.bind(this);
    this._processData = this._processData.bind(this);
    // modal related
    this.handleOpenModalAbout = this.handleOpenModalAbout.bind(this);
    this.handleCloseModalAbout = this.handleCloseModalAbout.bind(this);
    this.handleOpenModalControls = this.handleOpenModalControls.bind(this);
    this.handleCloseModalControls = this.handleCloseModalControls.bind(this);
  }

  // deck.gl related

  _getData() {
    var loc = window.location;
    // js binding 
    var up = this;
    var uri = 'ws:';

    if (loc.protocol === 'https:') {
      uri = 'wss:';
    }
    uri += '//' + loc.host;
    uri += loc.pathname + 'ws';

    var ws = new WebSocket(uri)

    ws.onopen = function () {
      console.log('Connected')
    }

    ws.onmessage = function (ev) {
      var json = JSON.parse(ev.data);
      if (json.hasOwnProperty('command')) {
        if (json.command == "flush") {
          // Flushing the data
          D4.length = 0;
          this.setState({
            points: []
          });
        }
      } else {
        D4.push(json[0]);
        up._processData();
      }
    }
  }

  componentDidMount() {
    // js binding 
    var up = this;
    fetch(DAILY)
      .then(response => response.text())
      .then((data) => {
        if (data) {
          var events = data.split(/\r\n|\r|\n/g);
          events.forEach(function (event) {
            if (event) {
              var json = JSON.parse(event);
              D4.push(json[0]);
            }
          });
          up._processData();
        }
      }, (err) => {
        console.log('Could not fetch ' + DAILY + ' ' + err); // Erreur !
      });

    this._getData();
    this._processData();
    window.addEventListener('resize', this._resize);
    this._resize();
  }

  _processData() {
    if (D4) {
      const points = D4.reduce((accu, curr) => {
        accu.push({
          position: [Number(curr.geoip_lon), Number(curr.geoip_lat)],
        });
        return accu;
      }, []);
      // Add the point to the ARCS
      this.setState({
        points
      });
    }
  }

  componentWillUnmount() {
    window.removeEventListener('resize', this._resize);
  }

  _onViewportChange = (viewport) => {
    this.setState({
      viewport: { ...this.state.viewport, ...viewport }
    });
  }

  _resize = () => {
    this._onViewportChange({
      width: window.innerWidth,
      height: window.innerHeight
    });
  }

  // modal related
  handleOpenModalAbout() {
    this.closeAllModals()
    this.setState({ showModalAbout: true });
  }

  handleCloseModalAbout() {
    this.setState({ showModalAbout: false });
  }

  handleOpenModalControls() {
    this.closeAllModals()
    this.setState({ showModalControls: true });
  }

  handleCloseModalControls() {
    this.setState({ showModalControls: false });
  }

  closeAllModals() {
    this.setState({ showModalAbout: false });
    this.setState({ showModalControls: false });
  }

  render() {
    return (
      <>
        <div style={{ zIndex: '1' }}>
          <Navbar collapseOnSelect expand="lg" bg="dark" variant="dark" fixed="top">
            <Navbar.Brand href="">D4 attack map</Navbar.Brand>
            <Navbar.Toggle aria-controls="responsive-navbar-nav" />
            <Navbar.Collapse id="responsive-navbar-nav">
              <Nav className="mr-auto">
                <Nav.Link onClick={this.handleOpenModalAbout}>About</Nav.Link>
                <Nav.Link onClick={this.handleOpenModalControls}>Controls</Nav.Link>
                <NavDropdown title="Statistics" id="collasible-nav-dropdown">
                  <NavDropdown.Item href="sshstatistics/dailystatistics.html">Daily</NavDropdown.Item>
                  <NavDropdown.Item href="sshstatistics/monthlystatistics.html">Monthly</NavDropdown.Item>
                  <NavDropdown.Item href="sshstatistics/yearlystatistics.html">Yearly</NavDropdown.Item>
                </NavDropdown>
              </Nav>
            </Navbar.Collapse>
          </Navbar>
        </div>
        <ReactModal
          isOpen={this.state.showModalAbout}
          contentLabel="onRequestClose Example"
          onRequestClose={this.handleCloseModal}
          style={MODALSTYLE}
        >
          <h2>About this page</h2>
          <p class="text-justify">This page displays realtime SSH bruteforce attacks registered against <a href="http://d4-project.org">d4-project's</a> main instance, hosted in Luxembourg.</p>
          <p class="text-justify">This page flushed every day at midnight.</p>

          <button onClick={this.handleCloseModalAbout}>Close</button>
        </ReactModal>
        <ReactModal
          isOpen={this.state.showModalControls}
          contentLabel="onRequestClose Example"
          onRequestClose={this.handleCloseModalControls}
          style={MODALSTYLE}
        >
          <h2>On a Desktop</h2>
          <h3>Using the Mouse</h3>
          <ul>
            <li>Drag and Drop to move,</li>
            <li>Scrool to zoom,</li>
            <li>Double Click to zoom,</li>
            <li>Hold ctrl while dragging to rotate the view and change bearing.</li>
          </ul>
          <h3>Using the Keyboard</h3>
          <ul>
            <li>Use arrow keys to move the view,</li>
            <li>Hold ctrl to rotate hand change the bearing.</li>
          </ul>
          <h2>On a Mobile Phone</h2>
          <ul>
            <li>Move you finger to move,</li>
            <li>Pinch to zoom,</li>
            <li>Drag two fingers to rotate.</li>
          </ul>
          <button onClick={this.handleCloseModalControls}>Close</button>
        </ReactModal>
        <div className="deck-container">
          <DeckGL
            controller={{
              type: MapController,
              scrollZoom: true,
              dragPan: true,
              dragRotate: true,
              doubleClickZoom: true,
              touchZoom: true,
              touchRotate: true,
              keyboard: true
            }}
            initialViewState={this.state.viewport}
          >
            <GeoJsonLayer
              id="base-map"
              data={COUNTRIES}
              stroked={true}
              filled={true}
              lineWidthMinPixels={2}
              opacity={0.4}
              getLineColor={[60, 60, 60]}
              getFillColor={[200, 200, 200]}
            />
            <ArcLayer
              id="arcs"
              data={this.state.points}
              getSourcePosition={f => [6.1319346, 49.611621]}
              getTargetPosition={f => f.position}
              getSourceColor={[0, 128, 200]}
              getTargetColor={[200, 0, 80]}
              getWidth={24}
              pickable={true}
            />
          </DeckGL>
        </div>
      </>
    );
  }
}
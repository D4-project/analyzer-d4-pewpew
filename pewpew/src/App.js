/* global window */
import React, { Component } from 'react';
import DeckGL, { GeoJsonLayer, ArcLayer } from 'deck.gl';

const COUNTRIES = window.location + 'map/ne_50m_admin_0_scale_rank.geojson';
const DAILY = window.location + "daily.json";
const D4 = [];

export default class App extends Component {
  constructor(props) {
    super(props);
    this.state = {
      viewport: {
        width: window.innerWidth,
        height: window.innerHeight,
        longitude: 6.1319346,
        latitude: 49.611621,
        zoom: 4,
        maxZoom: 12
      },
      points: []
    };
    this._getData = this._getData.bind(this);
    this._processData = this._processData.bind(this);
  }

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
      if (json.hasOwnProperty('command')){
        if (json.command == "flush"){
        // Flushing the data
        D4.length = 0;
        up._processData();
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
      // Flushing points
    } else {
      this.setState({
        points: []
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

  render() {
    return (
      <DeckGL controller={true} initialViewState={this.state.viewport}>
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
          getWidth={1}
        />
      </DeckGL>
    );
  }
}
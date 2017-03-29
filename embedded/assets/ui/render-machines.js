/* Digital Rebar: Provision */
/* Copyright 2016, RackN */
/* License: Apache v2 */
/* jshint esversion: 6 */

class Machine extends React.Component {

  constructor(props) {
    super(props);

    this.toggleExpand = this.toggleExpand.bind(this);
  }

  // expands this subnet
  toggleExpand() {
    var machine = this.props.machine;
    machine._expand = !machine._expand;
    this.props.change(this.props.index, machine);
  }

  // renders the element
  render() {
    var machine = JSON.parse(JSON.stringify(this.props.machine));
    return (
      <tbody 
        className={(machine.updating ? 'updating-content' : '') + " " + (machine._expand ? "expanded" : "")}
        style={{
          position: "relative",
          backgroundColor: (machine._error ? '#fdd' : (machine._new ? "#dfd" : (machine._edited ? "#eee" : "#fff"))),
          borderBottom: "thin solid #ddd"
        }}>
        <tr>
          <td>
            {machine.Name}
          </td>
          <td>
            {machine.Address}
          </td>
          <td>
            {machine.BootEnv}
          </td>
          <td>
            {machine.Description}
          </td>
          <td>
            {machine.Uuid}
          </td>
        </tr>
      </tbody>
    );
  }
}

class Machines extends React.Component {
  constructor(props) {
    super(props);

    this.state = {
      machines: []
    };

    this.componentDidMount = this.componentDidMount.bind(this);
    this.addMachine = this.addMachine.bind(this);
  }
  
  // gets the machine json from the api
  getMachines() {
    return new Promise((resolve, reject) => {

      // get the interfaces from the api
      $.getJSON("../api/v3/machines", data => {
        resolve({
          machines: data,
        });
      }).fail(() => {
        reject("Failed getting Machines");
      });

    });
  }

  // get the machine once this component mounts
  componentDidMount() {
    this.getMachines().then(data => {
      this.setState({
        machines: data.machines,
      }, err => {
        // rejected ?? 
      });
    });
  }

  // called to create a new machine
  // allows some data other than defaults to be passed in
  addMachine(machine) {
  }

  // makes the post/put request to update the machine
  updateMachine(i) {
    var machine = this.state.machines[i];
    machine.updating = true;
    this.setState({machine: this.state.machines});

    $.ajax({
      type: (machine._new ? "POST" : "PUT"),
      dataType: "json",
      contentType: "application/json",
      url: "/api/v3/machines" + (machine._new ? "" : "/" + machine.Name),
      data: JSON.stringify(machine)
    }).done((resp) => {
      
      // update the machines list with our new interface
      var machines = this.state.machines.concat([]);

      resp.updating = false;
      resp._edited = false;
      resp._new = false;
      resp._error = false;
      resp._errorMessage = '';
      
      //  update the state
      machines[i] = resp;
      this.setState({
        machines: machines
      });

    }).fail((err) => {
      
      var machines = this.state.machines.concat([]);
      var machine = machines[i];
      machine.updating = false;
      machine._error = true;

      // If our error is from the backend
      if(err.responseText) {
        var response = JSON.parse(err.responseText);
        machine._errorMessage = "Error (" + err.status + "): " + response.Messages.join(", ");
      } else { // maybe the backend is down
        machine._errorMessage = err.status;
      }

      this.setState({
        machines: machines
      });
    });
  }

  render() {
    $('#machineCount').text(this.state.machines.length);
    return (
    <div>
      <h2 style={{display: 'flex', justifyContent: 'space-between'}}>
        <span>Machines</span>
        <span>
          <a target="_blank" href="http://rocket-skates.readthedocs.io/en/latest/doc/ui.html#machines">UI Help</a> | <a target="_blank" href="/swagger-ui/#/machines">API Help</a>
        </span>
      </h2>
      <table className="fullwidth input-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Address</th>
            <th>BootEnv</th>
            <th>Description</th>
            <th>Uuid</th>
          </tr>
        </thead>
        {this.state.machines.map((val, i) =>
          <Machine
            machine={val}
            key={val.Uuid}
            id={i}
          />
        )}
        <tfoot>
          <tr>
            <td colSpan="5" style={{textAlign: "center"}}>
              <button onClick={()=>this.addBootEnv({})}>New Machine</button>
            </td>
          </tr>
        </tfoot>
      </table>
    </div>
    );
  }
}

ReactDOM.render(<Machines />, machines);

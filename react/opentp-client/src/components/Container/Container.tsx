import { Alignment, Button, Icon, Menu, MenuItem, Navbar, Popover, Position } from "@blueprintjs/core";
import 'ag-grid-community/dist/styles/ag-grid.css';
import 'ag-grid-community/dist/styles/ag-theme-balham-dark.css';
import FlexLayout, { Layout, Model, TabNode } from "flexlayout-react";
import { Error } from "grpc-web";
import log from 'loglevel';
import React, { ReactNode } from 'react';
import { getGrpcErrorMessage } from "../../common/grpcUtilities";
import { ClientConfigServiceClient } from "../../serverapi/ClientconfigserviceServiceClientPb";
import { Config, GetConfigParameters, StoreConfigParams } from "../../serverapi/clientconfigservice_pb";
import { MarketDataServiceClient } from "../../serverapi/Market-data-serviceServiceClientPb";
import { Empty } from "../../serverapi/modelcommon_pb";
import { OrderMonitorClient } from "../../serverapi/OrdermonitorServiceClientPb";
import { CancelAllOrdersForOriginatorIdParams } from "../../serverapi/ordermonitor_pb";
import ListingServiceImpl, { ListingService } from "../../services/ListingService";
import OrderServiceImpl, { OrderService } from "../../services/OrderService";
import QuoteServiceImpl, { QuoteService } from "../../services/QuoteService";
import Executions from "../Executions";
import InstrumentListingWatch from "../InstrumentWatch/InstrumentListingWatch";
import Login from "../Login";
import MarketDepth from '../MarketDepth/MarketDepth';
import ChildOrderBlotter from "../OrderBlotter/ChildOrderBlotter";
import OrderHistoryBlotter from "../OrderBlotter/OrderHistoryBlotter";
import ParentOrderBlotter from "../OrderBlotter/ParentOrderBlotter";
import OrderTicket from '../OrderTicket/OrderTicket';
import ColumnChooserAgGrid from "../AgGrid/ColumnChooseAgGrid";
import { ListingContext, OrderContext } from "./Contexts";
import { AgGridColumnChooserController, ChildOrderBlotterController, ExecutionsController, OrderHistoryBlotterController, QuestionDialogController, TicketController, ViewNameDialogController } from "./Controllers";
import QuestionDialog from "./QuestionDialog";
import ViewNameDialog from "./ViewNameDialog";








interface ContainerState {
    model: Model | undefined
}

enum Views {
    OrderBlotter = "order-blotter",
    InstrumentListingWatch = "instrument-watch",
    MarketDepth = "market-depth",
    NavigationBar = "nav-bar",
}




export default class Container extends React.Component<any, ContainerState> {


    orderMonitorClient = new OrderMonitorClient(Login.grpcContext.serviceUrl, null, null)
    clientConfigServiceClient = new ClientConfigServiceClient(Login.grpcContext.serviceUrl, null, null)


    factory: (node: TabNode) => React.ReactNode;

    quoteService: QuoteService
    orderService: OrderService
    listingService: ListingService
    listingContext: ListingContext
    orderContext: OrderContext
    ticketController: TicketController
    childOrderBlotterController: ChildOrderBlotterController
    orderHistoryBlotterController: OrderHistoryBlotterController
    executionsController: ExecutionsController
    questionDialogController: QuestionDialogController
    viewNameDialogController: ViewNameDialogController
    agGridcolChooserController: AgGridColumnChooserController

    constructor(p: any, s: ContainerState) {
        super(p, s);

        this.listingService = new ListingServiceImpl()
        this.quoteService = new QuoteServiceImpl( new MarketDataServiceClient(Login.grpcContext.serviceUrl, null, null))
        this.orderService = new OrderServiceImpl()
        this.listingContext = new ListingContext()
        this.orderContext = new OrderContext()
        this.ticketController = new TicketController()
        this.childOrderBlotterController = new ChildOrderBlotterController()
        this.orderHistoryBlotterController = new OrderHistoryBlotterController()
        this.executionsController = new ExecutionsController()
        this.questionDialogController = new QuestionDialogController()
        this.viewNameDialogController = new ViewNameDialogController()
        this.agGridcolChooserController = new AgGridColumnChooserController()

        this.factory = (node: TabNode) => {
            var component = node.getComponent();

            if (this.state && this.state.model) {

                if (component === Views.OrderBlotter) {
                    return <ParentOrderBlotter ticketController={this.ticketController} colController={this.agGridcolChooserController} executionsController={this.executionsController} orderHistoryBlotterController={this.orderHistoryBlotterController} childOrderBlotterController={this.childOrderBlotterController} listingService={this.listingService} orderService={this.orderService} orderContext={this.orderContext} node={node} model={this.state.model} />;
                }
                if (component === Views.MarketDepth) {
                    return <MarketDepth colController={this.agGridcolChooserController} listingContext={this.listingContext} quoteService={this.quoteService} listingService={this.listingService} node={node} model={this.state.model}
                        ticketController={this.ticketController} />;
                }
                if (component === Views.InstrumentListingWatch) {
                    return <InstrumentListingWatch colController={this.agGridcolChooserController} listingService={this.listingService} ticketController={this.ticketController} listingContext={this.listingContext} quoteService={this.quoteService} node={node} model={this.state.model} />;
                }
                if (component === Views.NavigationBar) {
                    return <Navbar />;
                }
            } else {
                return <div>Model not set</div>
            }


        }


        let params = new GetConfigParameters()
        params.setUserid(Login.username)
        this.clientConfigServiceClient.getClientConfig(params, Login.grpcContext.grpcMetaData, (err: Error,
            response: Config) => {
            let layoutJson: {}
            if (err) {
                layoutJson = {
                    global: {},
                    borders: [],
                    layout: {}
                }
            } else {
                layoutJson = JSON.parse(response.getConfig());

            }

            let md = FlexLayout.Model.fromJson(layoutJson)

            this.setState({
                model: md
            })

        })


        this.onSave = this.onSave.bind(this);
        this.onCancelAllOrders = this.onCancelAllOrders.bind(this);
    }


    onSave() {

        this.questionDialogController.open("Save Layout?", "Save Layout", (response) => {
            if (response) {

                if (this.state && this.state.model) {
                    var jsonStr = JSON.stringify(this.state.model.toJson(), null, "\t");

                    let params = new StoreConfigParams()
                    params.setUserid(Login.username)
                    params.setConfig(jsonStr)
                    this.clientConfigServiceClient.storeClientConfig(params, Login.grpcContext.grpcMetaData, (err: Error,
                        response: Empty) => {
                        if (err) {
                            log.error("failed to store configuration:", err)
                        }
                    })
                }
            }

        })

    }

    onCancelAllOrders() {
        this.questionDialogController.open("Cancel all desk orders?", "Cancel All Desk Orders", (response: boolean) => {
            if (response) {
                var params = new CancelAllOrdersForOriginatorIdParams()
                params.setOriginatorid(Login.desk)

                this.orderMonitorClient.cancelAllOrdersForOriginatorId(params, Login.grpcContext.grpcMetaData, (err: Error,
                    response: Empty) => {

                    if (err) {

                        let msg = getGrpcErrorMessage(err, "Failed to cancel all orders")
                        log.error(msg)
                        alert(msg)

                    } else {
                        log.debug("cancelled all orders")
                    }

                })
            }
        })


    }

    public render() {


        const viewsMenu = (
            <Menu>
                <MenuItem icon="graph" text="Market Depth" onClick={() => this.viewNameDialogController.open(Views.MarketDepth, "Market Depth",
                    (this.refs.layout as Layout))} />
                <MenuItem icon="map" text="Instrument Watch" onClick={() => this.viewNameDialogController.open(Views.InstrumentListingWatch, "Instrument Watch",
                    (this.refs.layout as Layout))} />
                <MenuItem icon="th" text="Order Blotter" onClick={() => this.viewNameDialogController.open(Views.OrderBlotter, "Order Blotter",
                    (this.refs.layout as Layout))} />
            </Menu>
        );

        let contents: React.ReactNode = "loading ...";
        if (this.state && this.state.model) {
            contents = <FlexLayout.Layout
                iconFactory={(node: TabNode): ReactNode | undefined => {
                    switch (node.getComponent()) {
                        case Views.MarketDepth:
                            return <Icon icon="graph" style={{ paddingRight: 5 }}></Icon>
                        case Views.InstrumentListingWatch:
                            return <Icon icon="map" style={{ paddingRight: 5 }}></Icon>
                        case Views.OrderBlotter:
                            return <Icon icon="th" style={{ paddingRight: 5 }}></Icon>

                    }


                    return <div></div>
                }}

                ref="layout"
                model={this.state.model}
                factory={this.factory}
            />;
        }





        return (<div className="app" >

            <div className="toolbar" >
                <Navbar className="bp3-dark">
                    <Navbar.Group align={Alignment.LEFT}>
                        <Navbar.Heading>Open Trading Platform</Navbar.Heading>
                        <Navbar.Divider />
                        <Popover content={viewsMenu} position={Position.RIGHT_TOP}>
                            <Button minimal={true} icon="add-to-artifact" text="Add View..." />
                        </Popover>
                        <Button className="bp3-minimal" icon="floppy-disk" text="Save Layout" onClick={this.onSave} />
                    </Navbar.Group>
                </Navbar>
            </div>
            <div>
                <OrderTicket quoteService={this.quoteService} tickerController={this.ticketController} ></OrderTicket>
                <ChildOrderBlotter  childOrderBlotterController={this.childOrderBlotterController} orderService={this.orderService} listingService={this.listingService}></ChildOrderBlotter>
                <OrderHistoryBlotter  orderHistoryBlotterController={this.orderHistoryBlotterController} orderService={this.orderService} listingService={this.listingService}></OrderHistoryBlotter>
                <Executions executionsController={this.executionsController} orderService={this.orderService} listingService={this.listingService}></Executions>
                <QuestionDialog controller={this.questionDialogController}></QuestionDialog>
                <ViewNameDialog controller={this.viewNameDialogController}></ViewNameDialog>
                <ColumnChooserAgGrid controller={this.agGridcolChooserController}></ColumnChooserAgGrid>
            </div>

            <div className="contents">
                {contents}
            </div>
            <div className="toolbar" >
                <Navbar className="bp3-dark">
                    <Navbar.Group align={Alignment.LEFT}>
                        <Navbar.Heading>{Login.username + "@" + Login.desk}</Navbar.Heading>
                        <Navbar.Divider />
                    </Navbar.Group>
                    <Navbar.Group align={Alignment.RIGHT}>
                        <Button className="bp3-minimal" icon="delete" text="Cancel All Desk Orders" onClick={this.onCancelAllOrders} />
                    </Navbar.Group>
                </Navbar>
            </div>

  );

        </div>);


    }

}






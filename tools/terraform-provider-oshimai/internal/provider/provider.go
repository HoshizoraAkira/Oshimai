package provider

import (
	"context"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// oshimaiProvider configures a client used by every resource this provider exposes.
type oshimaiProvider struct{}

// oshimaiProviderModel mirrors the provider {} block's configurable arguments.
type oshimaiProviderModel struct {
	ServerURL types.String `tfsdk:"server_url"`
}

// client is the small HTTP client threaded into resources via their Configure method.
type client struct {
	baseURL    string
	httpClient *http.Client
}

func New() provider.Provider {
	return &oshimaiProvider{}
}

func (p *oshimaiProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "oshimai"
}

func (p *oshimaiProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages recurring GameDay chaos/load schedules on an Oshimai server.",
		Attributes: map[string]schema.Attribute{
			"server_url": schema.StringAttribute{
				Required:    true,
				Description: "Base URL of the Oshimai server, e.g. https://oshimai.internal.example.com",
			},
		},
	}
}

func (p *oshimaiProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg oshimaiProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := &client{baseURL: cfg.ServerURL.ValueString(), httpClient: http.DefaultClient}
	resp.ResourceData = c
}

func (p *oshimaiProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewGameDayScheduleResource,
	}
}

func (p *oshimaiProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

# Payment: Stripe, Braintree, Square

# =============================================================================
section "Payment: Stripe"
# =============================================================================
expect_deny 'stripe customers delete cus_12345'        'stripe customers delete'
expect_deny 'stripe products delete prod_12345'        'stripe products delete'
expect_deny 'stripe prices delete price_12345'         'stripe prices delete'
expect_deny 'stripe webhook_endpoints delete we_12345' 'stripe webhook_endpoints delete'
expect_deny 'stripe coupons delete COUPON20'            'stripe coupons delete'
expect_deny 'stripe api_keys roll rk_live_xxx'         'stripe api_keys roll'
expect_deny 'curl -X DELETE https://api.stripe.com/v1/customers/cus_12345' \
                                                       'stripe api DELETE'

# =============================================================================
section "Payment: Braintree"
# =============================================================================
expect_deny 'curl -X DELETE https://api.braintreegateway.com/merchants/id/customers/cust' \
                                                       'braintree api delete'

# =============================================================================
section "Payment: Square"
# =============================================================================
expect_deny 'curl -X DELETE https://connect.squareup.com/v2/catalog/object/abc' \
                                                       'square api delete catalog'

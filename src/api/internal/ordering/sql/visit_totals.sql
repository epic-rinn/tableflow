SELECT coalesce(sum(unit_price_satang * quantity) FILTER (WHERE state NOT IN ('rejected', 'cancelled')), 0)::bigint,
       count(*) FILTER (WHERE state NOT IN ('rejected', 'cancelled'))
FROM order_lines WHERE visit_id = $1
